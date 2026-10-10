package valueobj

// Kind 标记采集值类型
type Kind string

const (
	KindNumber Kind = "number"
	KindText   Kind = "text"
	KindBool   Kind = "bool"
)

// CollectedValue 统一采集值模型
// 仅使用与 Kind 对应的字段。
type CollectedValue struct {
	Kind   Kind    `json:"kind"`
	Number float64 `json:"number,omitempty"`
	Text   string  `json:"text,omitempty"`
	Bool   bool    `json:"bool,omitempty"`
}

func NewNumberValue(v float64) CollectedValue {
	return CollectedValue{Kind: KindNumber, Number: v}
}

func NewTextValue(v string) CollectedValue {
	return CollectedValue{Kind: KindText, Text: v}
}

func NewBoolValue(v bool) CollectedValue {
	return CollectedValue{Kind: KindBool, Bool: v}
}

func (v CollectedValue) AsAny() interface{} {
	switch v.Kind {
	case KindText:
		return v.Text
	case KindBool:
		return v.Bool
	default:
		return v.Number
	}
}

