package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"

	"github.com/fridiculous/the-score/internal/api"
	"github.com/fridiculous/the-score/internal/ipc"
	"github.com/fridiculous/the-score/internal/model"
)

type Client struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	next int
}

func Dial() (*Client, string, error) {
	conn, address, err := ipc.DialDefault()
	if err != nil {
		return nil, address, err
	}
	return &Client{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(bufio.NewReader(conn)),
		next: 1,
	}, address, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Call(method string, params interface{}, result interface{}) error {
	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return err
		}
		rawParams = data
	}
	id, _ := json.Marshal(c.next)
	c.next++
	if err := c.enc.Encode(api.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}); err != nil {
		return err
	}
	var resp api.Response
	if err := c.dec.Decode(&resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}
	if result == nil {
		return nil
	}
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

func (c *Client) SubscribeEvents(since int64) (<-chan model.Event, error) {
	id, _ := json.Marshal(c.next)
	c.next++
	params, _ := json.Marshal(map[string]int64{"since": since})
	if err := c.enc.Encode(api.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "events/subscribe",
		Params:  params,
	}); err != nil {
		return nil, err
	}
	var resp api.Response
	if err := c.dec.Decode(&resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	out := make(chan model.Event, 128)
	go func() {
		defer close(out)
		for {
			var notif api.Notification
			if err := c.dec.Decode(&notif); err != nil {
				return
			}
			if notif.Method != "events/event" {
				continue
			}
			data, err := json.Marshal(notif.Params)
			if err != nil {
				continue
			}
			var event model.Event
			if err := json.Unmarshal(data, &event); err != nil {
				continue
			}
			out <- event
		}
	}()
	return out, nil
}
