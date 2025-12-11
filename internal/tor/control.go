package tor

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	address  string
	password string
	conn     net.Conn
	mu       sync.Mutex
}

type Status struct {
	Version            string
	BootstrapPhase     int
	CircuitEstablished bool
	NumCircuits        int
	Traffic            TrafficStats
}

type TrafficStats struct {
	BytesRead    int64
	BytesWritten int64
}

func NewClient(address, password string) *Client {
	return &Client{
		address:  address,
		password: password,
	}
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	conn, err := net.DialTimeout("tcp", c.address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to tor control port: %w", err)
	}

	c.conn = conn

	if c.password != "" {
		if err := c.authenticate(); err != nil {
			if closeErr := c.conn.Close(); closeErr != nil {
				return fmt.Errorf("authentication failed: %w (close error: %v)", err, closeErr)
			}
			c.conn = nil
			return err
		}
	}

	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *Client) authenticate() error {
	cmd := fmt.Sprintf("AUTHENTICATE \"%s\"\r\n", c.password)
	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("failed to send authenticate command: %w", err)
	}

	reader := bufio.NewReader(c.conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read authenticate response: %w", err)
	}

	if !strings.HasPrefix(response, "250") {
		return fmt.Errorf("authentication failed: %s", strings.TrimSpace(response))
	}

	return nil
}

func (c *Client) GetInfo(keys ...string) (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	cmd := fmt.Sprintf("GETINFO %s\r\n", strings.Join(keys, " "))
	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		if closeErr := c.conn.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to send getinfo command: %w (close error: %v)", err, closeErr)
		}
		c.conn = nil
		return nil, fmt.Errorf("failed to send getinfo command: %w", err)
	}

	reader := bufio.NewReader(c.conn)
	result := make(map[string]string)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if closeErr := c.conn.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to read getinfo response: %w (close error: %v)", err, closeErr)
			}
			c.conn = nil
			return nil, fmt.Errorf("failed to read getinfo response: %w", err)
		}

		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "250 OK") {
			break
		}

		if strings.HasPrefix(line, "250-") {
			line = strings.TrimPrefix(line, "250-")
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		} else if strings.HasPrefix(line, "250+") {
			key := strings.TrimSuffix(strings.TrimPrefix(line, "250+"), "=")
			var value strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					if closeErr := c.conn.Close(); closeErr != nil {
						return nil, fmt.Errorf("failed to read multiline data: %w (close error: %v)", err, closeErr)
					}
					c.conn = nil
					return nil, fmt.Errorf("failed to read multiline data: %w", err)
				}
				dataLine = strings.TrimSpace(dataLine)
				if dataLine == "." {
					break
				}
				value.WriteString(dataLine)
				value.WriteString("\n")
			}
			result[key] = strings.TrimSpace(value.String())
		}
	}

	return result, nil
}

func (c *Client) GetStatus() (*Status, error) {
	if err := c.Connect(); err != nil {
		return nil, err
	}

	info, err := c.GetInfo(
		"version",
		"status/bootstrap-phase",
		"status/circuit-established",
		"traffic/read",
		"traffic/written",
	)
	if err != nil {
		return nil, err
	}

	status := &Status{
		Version: info["version"],
	}

	if phase, ok := info["status/bootstrap-phase"]; ok {
		if fields := strings.Fields(phase); len(fields) > 0 {
			for _, field := range fields {
				if strings.HasPrefix(field, "PROGRESS=") {
					progressStr := strings.TrimPrefix(field, "PROGRESS=")
					if progress, err := strconv.Atoi(progressStr); err == nil {
						status.BootstrapPhase = progress
					}
				}
			}
		}
	}

	if established, ok := info["status/circuit-established"]; ok {
		status.CircuitEstablished = established == "1"
	}

	if read, ok := info["traffic/read"]; ok {
		if val, err := strconv.ParseInt(read, 10, 64); err == nil {
			status.Traffic.BytesRead = val
		}
	}

	if written, ok := info["traffic/written"]; ok {
		if val, err := strconv.ParseInt(written, 10, 64); err == nil {
			status.Traffic.BytesWritten = val
		}
	}

	return status, nil
}

func (c *Client) IsReady() bool {
	status, err := c.GetStatus()
	if err != nil {
		return false
	}
	return status.BootstrapPhase >= 100
}

func (c *Client) Signal(sig string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	cmd := fmt.Sprintf("SIGNAL %s\r\n", sig)
	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		if closeErr := c.conn.Close(); closeErr != nil {
			return fmt.Errorf("failed to send signal: %w (close error: %v)", err, closeErr)
		}
		c.conn = nil
		return fmt.Errorf("failed to send signal: %w", err)
	}

	reader := bufio.NewReader(c.conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		if closeErr := c.conn.Close(); closeErr != nil {
			return fmt.Errorf("failed to read signal response: %w (close error: %v)", err, closeErr)
		}
		c.conn = nil
		return fmt.Errorf("failed to read signal response: %w", err)
	}

	if !strings.HasPrefix(response, "250") {
		return fmt.Errorf("signal failed: %s", strings.TrimSpace(response))
	}

	return nil
}
