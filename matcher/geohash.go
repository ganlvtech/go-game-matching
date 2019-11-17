package matcher

// 内存结构大致是一个二维的表，每个单元内有一系列元素

// 返回 true 则终止迭代器
type GeoHashIterFunc func(v interface{}, i int, j int) bool

// 二维一级线性映射 Hash 表
// 这是一个专门为匹配队列设计的二维 Hash 表，不具有通用性
// GeoHash 并没有真正打散，而是分段连续的映射
type GeoHash struct {
	Data [][][]interface{}
	XCount    int
	YCount    int
	XGroupLen int
	YGroupLen int
}

func NewGeoHash(xCount int, yCount int, xGroupLen int, yGroupLen int) *GeoHash {
	h := &GeoHash{
		XCount:    xCount,
		YCount:    yCount,
		XGroupLen: xGroupLen,
		YGroupLen: yGroupLen,
	}
	h.Data = make([][][]interface{}, xCount)
	for i := range h.Data {
		h.Data[i] = make([][]interface{}, yCount)
		for j := range h.Data[i] {
			h.Data[i][j] = make([]interface{}, 0)
		}
	}
	return h
}

func (h *GeoHash) XLen() int {
	return h.XCount * h.XGroupLen
}
func (h *GeoHash) YLen() int {
	return h.YCount * h.YGroupLen
}
func (h *GeoHash) GetXGroupIndex(x int) int {
	return x / h.XGroupLen
}
func (h *GeoHash) GetYGroupIndex(y int) int {
	return y / h.YGroupLen
}
func (h *GeoHash) GetGroup(x int, y int) []interface{} {
	i := h.GetXGroupIndex(x)
	j := h.GetYGroupIndex(y)
	return h.Data[i][j]
}
func (h *GeoHash) Add(x int, y int, item interface{}) {
	i := h.GetXGroupIndex(x)
	j := h.GetYGroupIndex(y)
	h.Data[i][j] = append(h.Data[i][j], item)
}
func (h *GeoHash) Del(x int, y int, item interface{}) {
	i := h.GetXGroupIndex(x)
	j := h.GetYGroupIndex(y)
	for k, v := range h.Data[i][j] {
		if v == item {
			// 不是最后一个，则把最后一个拿到前面来
			if k != len(h.Data[i][j])-1 {
				h.Data[i][j][k] = h.Data[i][j][len(h.Data[i][j])-1]
			}
			// 删除掉最后一个
			h.Data[i][j] = h.Data[i][j][:len(h.Data[i][j])-1]
			break
		}
	}
}
