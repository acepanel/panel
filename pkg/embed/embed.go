package embed

import "embed"

//go:embed all:frontend/*
var PublicFS embed.FS

//go:embed all:website/*
var WebsiteFS embed.FS

//go:embed all:locales/*
var LocalesFS embed.FS

//go:embed all:error/*
var ErrorFS embed.FS
