package exp

var mhReplaceValue string

func init() {
	mhReplaceValue = ""
}

func setReplaceMHWith(value string) {
	mhReplaceValue = value
}

// MHReplaceValue returns a new value to replace MH LVSSheduler.
func MHReplaceValue() string {
	return mhReplaceValue
}
