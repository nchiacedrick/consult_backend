package data

import "consult_app.cedrickewi/internal/validator"

type Filters struct {
	Page     int
	PageSize int
	Sort     string
	SortSafe []string
}

func ValidateFilters(v *validator.Validator, f Filters) {
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be no more than 100")
	v.Check(validator.In(f.Sort, f.SortSafe...), "sort", "invalid sort value")
}