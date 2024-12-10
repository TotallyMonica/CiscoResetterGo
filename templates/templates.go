package templates

import _ "embed"

var Client = ``

var JobApi = ``

//go:embed device.html
var Device string

//go:embed index.html
var Index string

//go:embed job.html
var Job string

//go:embed jobs.html
var Jobs string

//go:embed layout.html
var Layout string

//go:embed port.html
var Port string

//go:embed ports.html
var Ports string

//go:embed reset.html
var Reset string
