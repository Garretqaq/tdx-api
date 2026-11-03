package protocol

import (
	"bufio"
	"errors"
	"net"
	"sync"
	"time"
)

// Conn 单条通达信 TCP 连接。同一时刻只处理一个请求（请求-响应串行），
// 因此无需按 MsgID 匹配响应；IO 出错（含超时）后连接视为损坏并关闭，由上层重建。
type Conn struct {
	mu      sync.Mutex
	nc      net.Conn
	r       *bufio.Reader
	msgID   uint32
	timeout time.Duration
	addr    string
	closed  bool
}

// Dial 连接到 addr（形如 "124.71.187.122:7709"）并完成握手。
func Dial(addr string, timeout time.Duration) (*Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	nc, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	c := &Conn{nc: nc, r: bufio.NewReaderSize(nc, 64*1024), timeout: timeout, addr: addr}
	if _, err := c.Do(TypeConnect, []byte{0x01}); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Addr 返回服务器地址。
func (c *Conn) Addr() string { return c.addr }

// Closed 连接是否已关闭或损坏。
func (c *Conn) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// Do 发送一个请求并返回解压后的响应数据域。
func (c *Conn) Do(typ uint16, data []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("连接已关闭")
	}
	c.msgID++
	f := &Frame{MsgID: c.msgID, Control: Control01, Type: typ, Data: data}
	_ = c.nc.SetDeadline(time.Now().Add(c.timeout))
	if _, err := c.nc.Write(f.Bytes()); err != nil {
		c.closeLocked()
		return nil, err
	}
	raw, err := ReadResponse(c.r)
	if err != nil {
		c.closeLocked()
		return nil, err
	}
	// 整帧已读完，解析失败（如服务器返回错误控制码）不会破坏流同步，无需关闭连接。
	resp, err := DecodeResponse(raw)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// Close 关闭连接。
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeLocked()
}

func (c *Conn) closeLocked() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.nc.Close()
}
