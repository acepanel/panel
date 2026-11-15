package request

type ToolboxBenchmarkTest struct {
	Name string `json:"name" validate:"required && in:image,matrix,sort,encryption,compression,physics,json,memory,disk"`
}
