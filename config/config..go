package config

import "time"

type Config struct {
	Timeout                 TimeDuration `yaml:"Timeout" json:"timeout"`
	Alpha                   float64      `yaml:"Alpha" json:"alpha"`
	DefaultPeakEWMADuration TimeDuration `yaml:"DefaultPeakEWMADuration" json:"default_peak_ewma_duration"`
	PickTimes               int          `yaml:"PickTimes" json:"pick_times"`
	EnableHealth            bool         `yaml:"EnableHealth" json:"enable_health"`
	EnableDuration          bool         `yaml:"EnableDuration" json:"enable_duration"`
	EnablePending           bool         `yaml:"EnablePending" json:"enable_pending"`
	EnableQPS               bool         `yaml:"EnableQPS" json:"enable_qps"`
	ErrorCodeList           []uint32     `yaml:"ErrorCodeList" json:"error_code_list"`
	EnableTick              bool         `yaml:"EnableTick" json:"enable_tick"`
	ActiveRequestBias       float64      `yaml:"ActiveRequestBias" json:"active_request_bias"`
	QPSBias                 float64      `yaml:"QPSBias" json:"qps_bias"`
}

func NewConfig() *Config {
	return &Config{
		Timeout:                 TimeDuration(10 * time.Minute),
		Alpha:                   0.1536,
		DefaultPeakEWMADuration: TimeDuration(5 * time.Second),
		PickTimes:               3,
		EnableHealth:            true,
		EnableDuration:          true,
		EnableQPS:               true,
		EnablePending:           true,
		ErrorCodeList:           []uint32{},
		EnableTick:              true,
		ActiveRequestBias:       1.0,
		QPSBias:                 10.0,
	}
}
