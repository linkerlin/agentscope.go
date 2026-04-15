package memory

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// QdrantVectorStore 基于 Qdrant 的远程向量存储实现
type QdrantVectorStore struct {
	client     *qdrant.Client
	collection string
	embed      EmbeddingModel
	dim        uint64
	mu         sync.RWMutex
}

// NewQdrantVectorStore 创建 Qdrant 向量存储并确保集合已存在
func NewQdrantVectorStore(host string, port int, collection string, dim uint64, embed EmbeddingModel) (*QdrantVectorStore, error) {
	if embed == nil {
		return nil, ErrEmbeddingRequired
	}
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, err
	}
	s := &QdrantVectorStore{
		client:     client,
		collection: collection,
		embed:      embed,
		dim:        dim,
	}
	if err := s.ensureCollection(context.Background()); err != nil {
		_ = s.client.Close()
		return nil, err
	}
	return s, nil
}

func (s *QdrantVectorStore) ensureCollection(ctx context.Context) error {
	exists, err := s.client.CollectionExists(ctx, s.collection)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: s.collection,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     s.dim,
			Distance: qdrant.Distance_Cosine,
		}),
	})
}

// Close 关闭底层连接
func (s *QdrantVectorStore) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

// Insert 插入记忆节点
func (s *QdrantVectorStore) Insert(ctx context.Context, nodes []*MemoryNode) error {
	if s.embed == nil {
		return ErrEmbeddingRequired
	}
	points, err := s.nodesToPoints(ctx, nodes)
	if err != nil {
		return err
	}
	if len(points) == 0 {
		return nil
	}
	wait := true
	_, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Wait:           &wait,
		Points:         points,
	})
	return err
}

// Search 语义检索
func (s *QdrantVectorStore) Search(ctx context.Context, query string, opts RetrieveOptions) ([]*MemoryNode, error) {
	if s.embed == nil {
		return nil, ErrEmbeddingRequired
	}
	qv, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	limit := uint64(opts.TopK)
	if limit <= 0 {
		limit = 10
	}

	filter := buildQdrantFilter(opts)
	scoreThreshold := float32(opts.MinScore)

	qps := &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQuery(qv...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(true),
	}
	if filter != nil {
		qps.Filter = filter
	}
	if opts.MinScore > 0 {
		qps.ScoreThreshold = &scoreThreshold
	}

	results, err := s.client.Query(ctx, qps)
	if err != nil {
		return nil, err
	}

	nodes := make([]*MemoryNode, 0, len(results))
	for _, r := range results {
		n := scoredPointToNode(r)
		if n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// Get 按 memoryID 读取
func (s *QdrantVectorStore) Get(ctx context.Context, memoryID string) (*MemoryNode, error) {
	id, err := memoryIDToQdrantID(memoryID)
	if err != nil {
		return nil, err
	}
	pts, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection,
		Ids:            []*qdrant.PointId{id},
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(true),
	})
	if err != nil {
		return nil, err
	}
	if len(pts) == 0 {
		return nil, ErrMemoryNotFound
	}
	n := retrievedPointToNode(pts[0])
	if n == nil {
		return nil, ErrMemoryNotFound
	}
	return n, nil
}

// Update 覆盖更新
func (s *QdrantVectorStore) Update(ctx context.Context, node *MemoryNode) error {
	points, err := s.nodesToPoints(ctx, []*MemoryNode{node})
	if err != nil {
		return err
	}
	if len(points) == 0 {
		return ErrInvalidMemoryNode
	}
	wait := true
	_, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Wait:           &wait,
		Points:         points,
	})
	return err
}

// Delete 按 memoryID 删除
func (s *QdrantVectorStore) Delete(ctx context.Context, memoryID string) error {
	id, err := memoryIDToQdrantID(memoryID)
	if err != nil {
		return err
	}
	wait := true
	_, err = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection,
		Wait:           &wait,
		Points:         qdrant.NewPointsSelector(id),
	})
	return err
}

// DeleteAll 清空集合（删除后重建）
func (s *QdrantVectorStore) DeleteAll(ctx context.Context) error {
	if err := s.client.DeleteCollection(ctx, s.collection); err != nil {
		return err
	}
	return s.ensureCollection(ctx)
}

// nodesToPoints 将 MemoryNode 列表转为 Qdrant PointStruct
func (s *QdrantVectorStore) nodesToPoints(ctx context.Context, nodes []*MemoryNode) ([]*qdrant.PointStruct, error) {
	var points []*qdrant.PointStruct
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if node.MemoryID == "" {
			node.MemoryID = GenerateMemoryID(node.Content)
		}
		if len(node.Vector) == 0 {
			v, err := s.embed.Embed(ctx, node.Content)
			if err != nil {
				return nil, err
			}
			node.Vector = v
		}
		id, err := memoryIDToQdrantID(node.MemoryID)
		if err != nil {
			return nil, err
		}
		payload := nodeToPayload(node)
		points = append(points, &qdrant.PointStruct{
			Id:      id,
			Payload: payload,
			Vectors: qdrant.NewVectors(node.Vector...),
		})
	}
	return points, nil
}

func buildQdrantFilter(opts RetrieveOptions) *qdrant.Filter {
	var must []*qdrant.Condition
	if len(opts.MemoryTypes) > 0 {
		types := make([]string, len(opts.MemoryTypes))
		for i, t := range opts.MemoryTypes {
			types[i] = string(t)
		}
		must = append(must, qdrant.NewMatchKeywords("memory_type", types...))
	}
	if len(opts.MemoryTargets) > 0 {
		must = append(must, qdrant.NewMatchKeywords("memory_target", opts.MemoryTargets...))
	}
	if len(must) == 0 {
		return nil
	}
	return &qdrant.Filter{Must: must}
}

func nodeToPayload(node *MemoryNode) map[string]*qdrant.Value {
	p := map[string]*qdrant.Value{
		"content":       qdrant.NewValueString(node.Content),
		"memory_type":   qdrant.NewValueString(string(node.MemoryType)),
		"memory_target": qdrant.NewValueString(node.MemoryTarget),
		"when_to_use":   qdrant.NewValueString(node.WhenToUse),
		"author":        qdrant.NewValueString(node.Author),
	}
	if !node.TimeCreated.IsZero() {
		p["time_created"] = qdrant.NewValueString(node.TimeCreated.Format(time.RFC3339))
	}
	if !node.TimeModified.IsZero() {
		p["time_modified"] = qdrant.NewValueString(node.TimeModified.Format(time.RFC3339))
	}
	if node.RefMemoryID != "" {
		p["ref_memory_id"] = qdrant.NewValueString(node.RefMemoryID)
	}
	if !node.MessageTime.IsZero() {
		p["message_time"] = qdrant.NewValueString(node.MessageTime.Format(time.RFC3339))
	}
	return p
}

func payloadToNode(payload map[string]*qdrant.Value, vectors []float32, score float64) *MemoryNode {
	getStr := func(key string) string {
		if v, ok := payload[key]; ok && v != nil {
			return v.GetStringValue()
		}
		return ""
	}
	parseTime := func(key string) time.Time {
		s := getStr(key)
		if s == "" {
			return time.Time{}
		}
		t, _ := time.Parse(time.RFC3339, s)
		return t
	}
	n := &MemoryNode{
		MemoryID:     getStr("memory_id"),
		Content:      getStr("content"),
		MemoryType:   MemoryType(getStr("memory_type")),
		MemoryTarget: getStr("memory_target"),
		WhenToUse:    getStr("when_to_use"),
		Author:       getStr("author"),
		RefMemoryID:  getStr("ref_memory_id"),
		TimeCreated:  parseTime("time_created"),
		TimeModified: parseTime("time_modified"),
		Vector:       vectors,
		Score:        score,
	}
	n.MessageTime = parseTime("message_time")
	return n
}

func scoredPointToNode(sp *qdrant.ScoredPoint) *MemoryNode {
	if sp == nil {
		return nil
	}
	vectors := extractVectorsOutput(sp.GetVectors())
	n := payloadToNode(sp.GetPayload(), vectors, float64(sp.GetScore()))
	if n.MemoryID == "" {
		n.MemoryID = qdrantIDToMemoryID(sp.GetId())
	}
	return n
}

func retrievedPointToNode(rp *qdrant.RetrievedPoint) *MemoryNode {
	if rp == nil {
		return nil
	}
	vectors := extractVectorsOutput(rp.GetVectors())
	n := payloadToNode(rp.GetPayload(), vectors, 0)
	if n.MemoryID == "" {
		n.MemoryID = qdrantIDToMemoryID(rp.GetId())
	}
	return n
}

func extractVectors(v *qdrant.Vectors) []float32 {
	if v == nil {
		return nil
	}
	vec := v.GetVector()
	if vec == nil {
		return nil
	}
	data := vec.GetData()
	out := make([]float32, len(data))
	for i, d := range data {
		out[i] = d
	}
	return out
}

func extractVectorsOutput(v *qdrant.VectorsOutput) []float32 {
	if v == nil {
		return nil
	}
	vec := v.GetVector()
	if vec == nil {
		return nil
	}
	data := vec.GetData()
	out := make([]float32, len(data))
	for i, d := range data {
		out[i] = d
	}
	return out
}

func memoryIDToQdrantID(memoryID string) (*qdrant.PointId, error) {
	num, err := strconv.ParseUint(memoryID, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("memory: invalid memoryID %q: %w", memoryID, err)
	}
	return qdrant.NewIDNum(num), nil
}

func qdrantIDToMemoryID(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	return fmt.Sprintf("%016x", id.GetNum())
}

var _ VectorStore = (*QdrantVectorStore)(nil)
