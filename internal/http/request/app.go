package request

type App struct {
	Slug    string `json:"slug" form:"slug" validate:"required|notExists:apps,slug"`
	Channel string `json:"channel" form:"channel" validate:"required"`
}

type AppSlug struct {
	Slug string `json:"slug" form:"slug" validate:"required"`
}

type AppSlugs struct {
	Slugs string `json:"slugs" form:"slugs" validate:"required"`
}

type AppUpdateShow struct {
	Slug string `json:"slug" form:"slug" validate:"required|exists:apps,slug"`
	Show bool   `json:"show" form:"show"`
}

type AppUpdateOrder struct {
	Slugs []string `json:"slugs" form:"slugs" validate:"required"`
}
