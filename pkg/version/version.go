package version

var version = "0.0.0"

func SetVersion(v string) {
	version = v
}

func GetVersion() string {
	return version
}
