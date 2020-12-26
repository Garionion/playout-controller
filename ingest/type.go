package ingest

type IngestType int

const (
	IcecastIngest IngestType = iota
	NginxRTMPIngest
)

type Source struct {
	Url string
	IngestType
}

type Icecast struct {
	Icestats *Icestats `json:"icestats"`
}
type IcecastSource struct {
	Genre              string      `json:"genre"`
	ListenerPeak       int         `json:"listener_peak"`
	Listeners          int         `json:"listeners"`
	Listenurl          string      `json:"listenurl"`
	ServerDescription  string      `json:"server_description"`
	ServerName         string      `json:"server_name"`
	ServerType         string      `json:"server_type"`
	StreamStart        string      `json:"stream_start"`
	StreamStartIso8601 string      `json:"stream_start_iso8601"`
	Dummy              interface{} `json:"dummy"`
}
type Icestats struct {
	Admin              string          `json:"admin"`
	Host               string          `json:"host"`
	Location           string          `json:"location"`
	ServerID           string          `json:"server_id"`
	ServerStart        string          `json:"server_start"`
	ServerStartIso8601 string          `json:"server_start_iso8601"`
	Source             []IcecastSource `json:"source"`
}

type RTMP struct {
	NginxVersion     string `xml:"nginx_version"`
	NginxRTMPVersion string `xml:"nginx_rtmp_version"`
	Built            string `xml:"built"`
	PID              int    `xml:"pid"`
	Uptime           int    `xml:"uptime"`
	Accepted         int    `xml:"naccepted"`
	BwIn             int    `xml:"bw_in"`
	BytesIn          int    `xml:"bytes_in"`
	BwOut            int    `xml:"bw_out"`
	BytesOut         int    `xml:"bytes_out"`
	Servers          struct {
		Application []Application `xml:"application"`
	} `xml:"server"`
}

type Application struct {
	Name string `xml:"name"`
	Live []struct {
		Stream rtmpStream `xml:"stream"`
	} `xml:"live"`
}

type rtmpStream struct {
	Name       string   `xml:"name"`
	Time       int      `xml:"time"`
	BwIn       int      `xml:"bw_in"`
	BytesIn    int      `xml:"bytes_in"`
	BwOut      int      `xml:"bw_out"`
	BytesOut   int      `xml:"bytes_out"`
	BwAudio    int      `xml:"bw_audio"`
	BwVideo    int      `xml:"bw_video"`
	Clients    []client `xml:"client"`
	Meta       meta     `xml:"meta"`
	Nclients   int      `xml:"nclients"`
	Publishing *bool    `xml:"publishing"`
	Active     *bool    `xml:"active"`
}

type client struct {
	ID         int    `xml:"id"`
	RemoteAddr string `xml:"address"`
	Time       int    `xml:"time"`
	Flashver   string `xml:"flashver"`
	Dropped    int    `xml:"dropped"`
	AVsync     int    `xml:"avsync"`
	Timestamp  int    `xml:"timestamp"`
	Active     *bool  `xml:"active"`
	Publishing *bool  `xml:"publishing"`
}

type meta struct {
	Video struct {
		Width     int     `xml:"width"`
		Height    int     `xml:"height"`
		Framerate int     `xml:"frame_rate"`
		Codec     string  `xml:"codec"`
		Profile   string  `xml:"profile"`
		Compat    int     `xml:"compat"`
		Level     float64 `xml:"level"`
	} `xml:"video"`
	Audio struct {
		Codec      string `xml:"codec"`
		Profile    string `xml:"profile"`
		Channels   int    `xml:"channels"`
		Samplerate int    `xml:"sample_rate"`
	} `xml:"audio"`
}
