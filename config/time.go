package config

import (
	"encoding/json"
	"time"
)

// Customized time.Duration, able to unmarshal from YAML and JSON
type TimeDuration time.Duration

func (d *TimeDuration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var durationStr string
	if err := unmarshal(&durationStr); err != nil {
		return err
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return err
	}

	*d = TimeDuration(duration)
	return nil
}

func (d *TimeDuration) UnmarshalJSON(data []byte) error {
	var durationStr string
	var durationNo int64

	// In order to be compatible with 2 types of duration format
	// We try to Unmarshal 2 times with different data type until it success
	// When all Agent upgrade to the latest version, try to change the configurations to standard format
	// Then delete the compatible logic.
	if err := json.Unmarshal(data, &durationStr); err != nil {
		if err2 := json.Unmarshal(data, &durationNo); err2 == nil {
			*d = TimeDuration(durationNo)
			return nil
		}
		return err
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return err
	}

	*d = TimeDuration(duration)
	return nil
}
