package conf

import "fmt"

type TXPacing struct {
	Enabled  bool `yaml:"enabled"`
	RateMbps int  `yaml:"rate_mbps"`
}

type TX struct {
	Retries int `yaml:"retries"`
	RetryUS int `yaml:"retry_us"`

	RawPacketRetries int `yaml:"raw_packet_retries"`
	RawPacketRetryUS int `yaml:"raw_packet_retry_us"`

	TCPWriteRetries    int `yaml:"tcp_write_retries"`
	TCPWriteRetryUS    int `yaml:"tcp_write_retry_us"`
	TCPWriteRetryMaxUS int `yaml:"tcp_write_retry_max_us"`

	Pacing TXPacing `yaml:"pacing"`
}

func (t *TX) setDefaults() {
	if t.Retries == 0 {
		t.Retries = 3
	}
	if t.RetryUS == 0 {
		t.RetryUS = 200
	}

	if t.RawPacketRetries == 0 {
		t.RawPacketRetries = t.Retries
	}
	if t.RawPacketRetryUS == 0 {
		t.RawPacketRetryUS = t.RetryUS
	}

	if t.TCPWriteRetries == 0 {
		t.TCPWriteRetries = 8
	}
	if t.TCPWriteRetryUS == 0 {
		t.TCPWriteRetryUS = 200
	}
	if t.TCPWriteRetryMaxUS == 0 {
		t.TCPWriteRetryMaxUS = 25000
	}
}

func (t *TX) validate() []error {
	var errors []error

	validateRetries := func(name string, v int) {
		if v < 0 {
			errors = append(errors, fmt.Errorf("network tx %s must be >= 0", name))
		}
		if v > 20 {
			errors = append(errors, fmt.Errorf("network tx %s too large (max 20)", name))
		}
	}
	validateRetryUS := func(name string, v int) {
		if v < 0 {
			errors = append(errors, fmt.Errorf("network tx %s must be >= 0", name))
		}
		if v > 1_000_000 {
			errors = append(errors, fmt.Errorf("network tx %s too large (max 1000000)", name))
		}
	}

	validateRetries("retries", t.Retries)
	validateRetryUS("retry_us", t.RetryUS)
	validateRetries("raw_packet_retries", t.RawPacketRetries)
	validateRetryUS("raw_packet_retry_us", t.RawPacketRetryUS)
	validateRetries("tcp_write_retries", t.TCPWriteRetries)
	validateRetryUS("tcp_write_retry_us", t.TCPWriteRetryUS)
	validateRetryUS("tcp_write_retry_max_us", t.TCPWriteRetryMaxUS)

	if t.Pacing.RateMbps < 0 {
		errors = append(errors, fmt.Errorf("network tx pacing rate_mbps must be >= 0"))
	}

	return errors
}
