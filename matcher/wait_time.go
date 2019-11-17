package matcher

const (
	GaussianBlurMinIndex  = -2
	GaussianBlurMaxIndex  = 2
)

var GaussianBlurFactors = []float64{1, 4, 6, 4, 1}

type WaitTime struct {
	groupCount            int       // 分组数
	Groups                []float64 // 分组等待时间
	groupBuffers          []float64 // 待计算等待时间
	groupBufferItemCounts []int     // 待计算分组的元素个数
	MaxWaitTime           float64   // 最长等待时间
	lastTime              float64   // 上次时间`
}

func NewWaitTime(count int, maxWaitTime float64) *WaitTime {
	w := &WaitTime{
		groupCount:            count,
		Groups:                make([]float64, count),
		groupBuffers:          make([]float64, count),
		groupBufferItemCounts: make([]int, count),
		MaxWaitTime:           maxWaitTime,
		lastTime:              -1,
	}
	for i := range w.Groups {
		w.Groups[i] = maxWaitTime
	}
	return w
}

func (w *WaitTime) GroupCount() int {
	return w.groupCount
}

func (w *WaitTime) AddItem(groupIndex int, t float64) {
	w.groupBuffers[groupIndex] += t
	w.groupBufferItemCounts[groupIndex]++
}

func (w *WaitTime) AddTime(deltaT float64) {
	for i := range w.Groups {
		w.Groups[i] += deltaT
	}
}

func (w *WaitTime) AddTimeAuto(t float64) {
	if w.lastTime >= 0 {
		w.AddTime(t - w.lastTime)
	}
	w.lastTime = t
}

func (w *WaitTime) Merge() {
	for i := range w.Groups {
		w.AddItem(i, w.Groups[i])
	}
	for i := range w.groupBuffers {
		sum := float64(0)
		sumFactor := float64(0)
		for j := GaussianBlurMinIndex; j <= GaussianBlurMaxIndex; j++ {
			k := i + j
			if k < 0 {
				k = 0
			} else if k >= w.groupCount {
				k = w.groupCount - 1
			}
			if w.groupBufferItemCounts[k] > 0 {
				sum += w.groupBuffers[k] * GaussianBlurFactors[j+2]
				sumFactor += float64(w.groupBufferItemCounts[k]) * GaussianBlurFactors[j+2]
			}
		}
		if sumFactor > 0 {
			w.Groups[i] = sum / sumFactor
		}
	}
	for i := range w.groupBuffers {
		w.groupBuffers[i] = 0
		w.groupBufferItemCounts[i] = 0
	}
	for i := range w.Groups {
		if w.Groups[i] > w.MaxWaitTime {
			w.Groups[i] = w.MaxWaitTime
		}
	}
}
