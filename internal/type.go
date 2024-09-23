// Package internal ...
package internal

const (
	// DefaultTimezone represents the default timezone for webhook applications
	DefaultTimezone = CSTTimezone
	// CSTTimezone is TZ database name for CST timezone
	CSTTimezone = "Asia/Shanghai"

	// InjectedAnnotation 记录是否已经注入时区，只有在第一次注入的时候回写这个 annotation
	InjectedAnnotation = "timezone.jugglechat.io/injected"
	// TimezoneAnnotation is set user timezone
	TimezoneAnnotation = "timezone.jugglechat.io/timezone"
	// InjectionStrategyAnnotation set injection strategy
	InjectionStrategyAnnotation = "timezone.jugglechat.io/strategy"
	// InjectAnnotation set inject
	InjectAnnotation = "timezone.jugglechat.io/inject"
)

// Patches Patch slince
type Patches []Patch

// Patch pod patch
type Patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}
