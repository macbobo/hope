package app

type Filter_t int

const (
	FILTER_NONE Filter_t = iota
	FILTER_ACCEPT
	FILTER_DROP
	FILER_RETJECT
	FILTER_CACHE
	FILTER_MODIFY
)
