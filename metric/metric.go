package metric

import (
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusResult struct {
	PromDesc      *prometheus.Desc
	PromValueType prometheus.ValueType
	Value         float64
	LabelValues   []string
}

// PromDesc  struct
type PromDesc struct {
	FqName      string            `json:"fqName"`
	Help        string            `json:"help"`
	VarLabels   []string          `json:"varLabels"`
	FixedLabels map[string]string `json:"fixedLabels"`
}

type ActionArg struct {
	Name           string `json:"Name"`
	IsIndex        bool   `json:"IsIndex"`
	ProviderAction string `json:"ProviderAction"`
	Value          string `json:"Value"`
}

// Metric struct
type Metric struct {
	PromDesc       PromDesc   `json:"promDesc"`
	PromType       string     `json:"promType"`
	ResultKey      string     `json:"resultKey"`
	OkValue        string     `json:"okValue"`
	ResultPath     string     `json:"resultPath"`
	Page           string     `json:"page"`
	Service        string     `json:"service"`
	Action         string     `json:"action"`
	ActionArgument *ActionArg `json:"actionArgument"`

	Desc        *prometheus.Desc
	Type        prometheus.ValueType
	Value       float64
	labelValues []string

	MetricResult []map[string]interface{} //filled during collect
	PromResult   []*PrometheusResult
}

// LabelRename struct
type LabelRename struct {
	MatchRegex  string `json:"matchRegex"`
	RenameLabel string `json:"renameLabel"`
	Pattern     regexp.Regexp
}

// MetricsFile struct
type MetricsFile struct {
	LabelRenames []*LabelRename `json:"labelRenames"`
	Metrics      []*Metric      `json:"metrics"`
}
