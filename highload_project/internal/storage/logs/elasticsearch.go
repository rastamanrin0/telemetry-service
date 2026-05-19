package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"logs-metrics-platform/config"
	"logs-metrics-platform/internal/domain"
)

const (
	activeIndex  = "logs-active"
	archiveIndex = "logs-archive"
)

type Repository interface {
	Save(ctx context.Context, log *domain.Log) error
	SaveBatch(ctx context.Context, logs []*domain.Log) error
	Search(ctx context.Context, query *domain.LogSearchQuery) (*domain.LogSearchResult, error)
	GetStats(ctx context.Context, query *domain.LogStatsQuery) (*domain.LogStats, error)
	ArchiveExpired(ctx context.Context, before time.Time) error
	DeleteExpired(ctx context.Context, policy domain.RetentionPolicy, before time.Time) error
}

type ElasticsearchRepository struct {
	client *elasticsearch.Client
}

func NewElasticsearchRepository(cfg config.ElasticConfig) (*ElasticsearchRepository, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.Addresses,
	})
	if err != nil {
		return nil, fmt.Errorf("creating elasticsearch client: %w", err)
	}
	repo := &ElasticsearchRepository{client: client}
	if err := repo.ensureIndices(context.Background()); err != nil {
		return nil, fmt.Errorf("ensuring indices: %w", err)
	}
	return repo, nil
}

func (r *ElasticsearchRepository) ensureIndices(ctx context.Context) error {
	mapping := `{
		"mappings": {
			"properties": {
				"id":               {"type": "keyword"},
				"timestamp":        {"type": "date"},
				"service_name":     {"type": "keyword"},
				"host_id":          {"type": "keyword"},
				"instance_id":      {"type": "keyword"},
				"level":            {"type": "keyword"},
				"message":          {"type": "text", "analyzer": "standard"},
				"retention_policy": {"type": "keyword"}
			}
		}
	}`
	for _, index := range []string{activeIndex, archiveIndex} {
		req := esapi.IndicesCreateRequest{
			Index: index,
			Body:  strings.NewReader(mapping),
		}
		res, err := req.Do(ctx, r.client)
		if err != nil {
			return fmt.Errorf("creating index %s: %w", index, err)
		}
		res.Body.Close()
		// 400 = index already exists, which is fine
	}
	return nil
}

func (r *ElasticsearchRepository) Save(ctx context.Context, log *domain.Log) error {
	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("marshaling log: %w", err)
	}
	req := esapi.IndexRequest{
		Index:      activeIndex,
		DocumentID: log.ID,
		Body:       bytes.NewReader(data),
		Refresh:    "false",
	}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("indexing log: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("elasticsearch index error [%s]: %s", log.ID, res.String())
	}
	return nil
}

func (r *ElasticsearchRepository) SaveBatch(ctx context.Context, logs []*domain.Log) error {
	if len(logs) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, l := range logs {
		meta := fmt.Sprintf(`{"index":{"_index":%q,"_id":%q}}`, activeIndex, l.ID)
		buf.WriteString(meta)
		buf.WriteByte('\n')
		data, err := json.Marshal(l)
		if err != nil {
			return fmt.Errorf("marshaling log %s: %w", l.ID, err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	req := esapi.BulkRequest{
		Body:    &buf,
		Refresh: "false",
	}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("bulk indexing logs: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("elasticsearch bulk error: %s", res.String())
	}
	return nil
}

func (r *ElasticsearchRepository) Search(ctx context.Context, q *domain.LogSearchQuery) (*domain.LogSearchResult, error) {
	filter := []interface{}{
		map[string]interface{}{
			"range": map[string]interface{}{
				"timestamp": map[string]interface{}{
					"gte": q.From.Format(time.RFC3339),
					"lte": q.To.Format(time.RFC3339),
				},
			},
		},
	}
	if q.ServiceName != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"service_name": q.ServiceName}})
	}
	if q.HostID != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"host_id": q.HostID}})
	}
	if q.Level != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"level": string(q.Level)}})
	}
	if q.RetentionPolicy != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"retention_policy": string(q.RetentionPolicy)}})
	}

	boolQ := map[string]interface{}{"filter": filter}
	if q.Query != "" {
		boolQ["must"] = []interface{}{
			map[string]interface{}{"match": map[string]interface{}{"message": q.Query}},
		}
	}

	size := q.Size
	if size <= 0 {
		size = 100
	}

	esQuery := map[string]interface{}{
		"query": map[string]interface{}{"bool": boolQ},
		"sort":  []interface{}{map[string]interface{}{"timestamp": map[string]interface{}{"order": "asc"}}},
		"from":  q.Page * size,
		"size":  size,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil, err
	}
	req := esapi.SearchRequest{
		Index: []string{activeIndex},
		Body:  &buf,
	}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch search: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch search error: %s", res.String())
	}
	return parseSearchResult(res.Body)
}

func (r *ElasticsearchRepository) GetStats(ctx context.Context, q *domain.LogStatsQuery) (*domain.LogStats, error) {
	filter := []interface{}{
		map[string]interface{}{
			"range": map[string]interface{}{
				"timestamp": map[string]interface{}{
					"gte": q.From.Format(time.RFC3339),
					"lte": q.To.Format(time.RFC3339),
				},
			},
		},
	}
	if q.ServiceName != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"service_name": q.ServiceName}})
	}

	esQuery := map[string]interface{}{
		"size":  0,
		"query": map[string]interface{}{"bool": map[string]interface{}{"filter": filter}},
		"aggs": map[string]interface{}{
			"by_level": map[string]interface{}{
				"terms": map[string]interface{}{"field": "level", "size": 10},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil, err
	}
	req := esapi.SearchRequest{
		Index: []string{activeIndex},
		Body:  &buf,
	}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch stats: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch stats error: %s", res.String())
	}
	return parseStatsResult(res.Body)
}

func (r *ElasticsearchRepository) ArchiveExpired(ctx context.Context, before time.Time) error {
	query := buildRetentionQuery(domain.RetentionPolicyArchive, before)

	reindexBody := map[string]interface{}{
		"source": map[string]interface{}{
			"index": activeIndex,
			"query": query,
		},
		"dest": map[string]interface{}{
			"index": archiveIndex,
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reindexBody); err != nil {
		return err
	}
	req := esapi.ReindexRequest{Body: &buf}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("reindex for archive: %w", err)
	}
	res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("reindex error: %s", res.String())
	}
	return r.deleteByQuery(ctx, activeIndex, query)
}

func (r *ElasticsearchRepository) DeleteExpired(ctx context.Context, policy domain.RetentionPolicy, before time.Time) error {
	return r.deleteByQuery(ctx, activeIndex, buildRetentionQuery(policy, before))
}

func (r *ElasticsearchRepository) deleteByQuery(ctx context.Context, index string, query map[string]interface{}) error {
	body := map[string]interface{}{"query": query}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return err
	}
	req := esapi.DeleteByQueryRequest{
		Index: []string{index},
		Body:  &buf,
	}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("delete by query: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("delete by query error: %s", res.String())
	}
	return nil
}

func buildRetentionQuery(policy domain.RetentionPolicy, before time.Time) map[string]interface{} {
	return map[string]interface{}{
		"bool": map[string]interface{}{
			"filter": []interface{}{
				map[string]interface{}{"term": map[string]interface{}{"retention_policy": string(policy)}},
				map[string]interface{}{
					"range": map[string]interface{}{
						"timestamp": map[string]interface{}{"lt": before.Format(time.RFC3339)},
					},
				},
			},
		},
	}
}

func parseSearchResult(body io.Reader) (*domain.LogSearchResult, error) {
	var raw struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source domain.Log `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}
	logs := make([]*domain.Log, len(raw.Hits.Hits))
	for i, h := range raw.Hits.Hits {
		entry := h.Source
		logs[i] = &entry
	}
	return &domain.LogSearchResult{Total: raw.Hits.Total.Value, Logs: logs}, nil
}

func parseStatsResult(body io.Reader) (*domain.LogStats, error) {
	var raw struct {
		Aggregations struct {
			ByLevel struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"by_level"`
		} `json:"aggregations"`
	}
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding stats response: %w", err)
	}
	counts := make(map[string]int64, len(raw.Aggregations.ByLevel.Buckets))
	for _, b := range raw.Aggregations.ByLevel.Buckets {
		counts[b.Key] = b.DocCount
	}
	return &domain.LogStats{Counts: counts}, nil
}
