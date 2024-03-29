package main

//Version mbstyle version
const Version = 8

const (
	//SourceTypeVector 矢量
	SourceTypeVector = "vector"
	//SourceTypeRaster 影像
	SourceTypeRaster = "raster"
	//SourceTypeGeoJSON geojson
	SourceTypeGeoJSON = "geojson"
	//SourceTypeImage 图像
	SourceTypeImage = "image"
	//SourceTypeVideo 视频
	SourceTypeVideo = "video"
	//SourceTypeCanvas canvas画布
	SourceTypeCanvas = "canvas"
)

//Source 数据源
type Source struct {
	Type string `json:"type"`
	// An array of one or more tile source URLs, as in the TileJSON spec.
	Tiles []string `json:"tiles,omitempty"`
	// defaults to 0 if not set
	MinZoom int `json:"minzoom,omitempty"`
	// defaults to 22 if not set
	MaxZoom int `json:"maxzoom,omitempty"`
	// url to TileJSON resource
	URL string `json:"url,omitempty"`
}

//Light light
type Light struct {
	Anchor    string  `json:"anchor"`
	Color     string  `json:"color"`
	Intensity float64 `json:"intensity"`
}

//Transition 变换
type Transition struct{}

//Root 根节点
// https://www.mapbox.com/mapbox-gl-js/style-spec/
type Root struct {
	// Style specification version number
	Version int `json:"version"`
	// A human-readable name for the style.
	Name string `json:"name,omitempty"`
	// Arbitrary properties useful to track with the stylesheet, but do not influence rendering.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Default map center in longitude and latitude. The style center will be
	// used only if the map has not been positioned by other means
	// (e.g. map options or user interaction).
	Center [2]float64 `json:"center,omitempty"`
	// Default zoom level. The style zoom will be used only if the map has not been
	// positioned by other means (e.g. map options or user interaction).
	Zoom float64 `json:"zoom,omitempty"`
	// Default bearing, in degrees clockwise from true north. The style bearing
	// will be used only if the map has not been positioned by other means
	// (e.g. map options or user interaction).
	Bearing *float64 `json:"bearing,omitempty"`
	// Default pitch, in degrees. Zero is perpendicular to the surface, for a
	// look straight down at the map, while a greater value like 60 looks ahead
	// towards the horizon. The style pitch will be used only if the map has not
	// been positioned by other means (e.g. map options or user interaction).
	Pitch *float64 `json:"pitch,omitempty"`
	// The global light source.
	Light *Light `json:"light,omitempty"`
	// Data source specifications.
	Sources map[string]*Source `json:"sources"`
	// URL to sprites. i.e. - mapbox://sprites/mapbox/streets-v8
	Sprite string `json:"sprite,omitempty"`
	// url to glyphs. i.e. - mapbox://fonts/mapbox/{fontstack}/{range}.pbf
	Glyphs string `json:"glyphs,omitempty"`
	// A global transition definition to use as a default across properties.
	Transition *Transition `json:"transition,omitempty"`
	// Layers will be drawn in the order of this array.
	Layers []interface{} `json:"layers"`
}
