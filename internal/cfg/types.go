package cfg

type Config struct {
	Global   GlobalConfig      `toml:"Global"`
	HTTP     HTTPConfig        `toml:"HTTP"`
	Admin    AdminConfig       `toml:"Admin"`
	SVN      SVNConfig         `toml:"SVN"`
	Audit    AuditConfig       `toml:"Audit"`
	Backup   BackupConfig      `toml:"Backup"`
	Session  SessionConfig     `toml:"Session"`
	Metadata MetadataConfig    `toml:"Metadata"`
	PathMap  map[string]string `toml:"PathMap"`
}

type GlobalConfig struct {
	Title   string `toml:"Title"`
	RunMode string `toml:"RunMode"`
}

type HTTPConfig struct {
	Host             string   `toml:"Host"`
	Port             int      `toml:"Port"`
	ReadTimeout      int      `toml:"ReadTimeout"`
	WriteTimeout     int      `toml:"WriteTimeout"`
	IdleTimeout      int      `toml:"IdleTimeout"`
	ShutdownTimeout  int      `toml:"ShutdownTimeout"`
	MaxContentLength int64    `toml:"MaxContentLength"`
	TrustedProxies   []string `toml:"TrustedProxies"`
}

type AdminConfig struct {
	Username string `toml:"Username"`
	Password string `toml:"Password"`
}

type SVNConfig struct {
	RepoRoot            string `toml:"RepoRoot"`
	PasswdFile          string `toml:"PasswdFile"`
	AuthzFile           string `toml:"AuthzFile"`
	SvnlookPath         string `toml:"SvnlookPath"`
	CommandTimeoutSec   int    `toml:"CommandTimeoutSeconds"`
	MaxCommandOutputMiB int64  `toml:"MaxCommandOutputMiB"`
	MaxPathsPerLevel    int    `toml:"MaxPathsPerLevel"`
}

type AuditConfig struct {
	Dir string `toml:"Dir"`
}

type BackupConfig struct {
	Dir  string `toml:"Dir"`
	Keep int    `toml:"Keep"`
}

type SessionConfig struct {
	IdleMinutes   int  `toml:"IdleMinutes"`
	AbsoluteHours int  `toml:"AbsoluteHours"`
	CookieSecure  bool `toml:"CookieSecure"`
}

type MetadataConfig struct {
	Departments []string `toml:"Departments"`
}
