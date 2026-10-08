package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"
)

// Minimal Discord IPC client. All pipe I/O happens on one goroutine in a strict
// request -> reply order, because Windows pipes opened by the os package are
// synchronous and a pending read would block writes.

const (
	opHandshake = 0
	opFrame     = 1
	opClose     = 2
	opPing      = 3
	opPong      = 4
)

type ipcCmd struct {
	setClient string
	activity  json.RawMessage
	isAct     bool
}

type ipc struct {
	mu        sync.Mutex
	connected bool
	user      string
	errText   string
	cmds      chan ipcCmd
	clientID  string
}

func newIPC(clientID string) *ipc {
	return &ipc{clientID: clientID, cmds: make(chan ipcCmd, 32)}
}

func (c *ipc) setActivity(act json.RawMessage) { c.cmds <- ipcCmd{activity: act, isAct: true} }
func (c *ipc) setClientID(id string)           { c.cmds <- ipcCmd{setClient: id} }
func (c *ipc) clear()                          { c.setActivity(nil); time.Sleep(300 * time.Millisecond) }

func (c *ipc) status() (bool, string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected, c.user, c.errText
}

func (c *ipc) setStatus(connected bool, user, errText string) {
	c.mu.Lock()
	c.connected, c.user, c.errText = connected, user, errText
	c.mu.Unlock()
}

func (c *ipc) loop() {
	var conn io.ReadWriteCloser
	var want json.RawMessage
	dirty := false
	clientID := c.clientID
	retry := time.NewTimer(time.Hour)
	nonce := 0

	closeConn := func(reason string) {
		if conn != nil {
			conn.Close()
			conn = nil
		}
		c.setStatus(false, "", reason)
	}

	for {
		select {
		case cmd := <-c.cmds:
			apply := func(cmd ipcCmd) {
				if cmd.setClient != "" && cmd.setClient != clientID {
					clientID = cmd.setClient
					closeConn("")
					dirty = true
				}
				if cmd.isAct {
					want = cmd.activity
					dirty = true
				}
			}
			apply(cmd)
			// Coalesce a burst of updates into the latest one.
			for drained := false; !drained; {
				select {
				case more := <-c.cmds:
					apply(more)
				default:
					drained = true
				}
			}
		case <-retry.C:
		}

		if !dirty {
			continue
		}
		if conn == nil {
			if want == nil {
				dirty = false // nothing to clear: we are not connected anyway
				continue
			}
			var user string
			var err error
			conn, user, err = dial(clientID)
			if err != nil {
				conn = nil
				c.setStatus(false, "", err.Error())
				retry.Reset(retryEvery)
				continue
			}
			c.setStatus(true, user, "")
		}

		nonce++
		args := map[string]any{"pid": pid()}
		if want != nil {
			args["activity"] = want
		}
		err := writeFrame(conn, opFrame, map[string]any{"cmd": "SET_ACTIVITY", "args": args, "nonce": strconv.Itoa(nonce)})
		if err == nil {
			var reply map[string]any
			reply, err = readReply(conn)
			if err == nil && reply["evt"] == "ERROR" {
				msg := "Discord 回報錯誤"
				if d, ok := reply["data"].(map[string]any); ok {
					msg = fmt.Sprintf("Discord：%v", d["message"])
				}
				c.mu.Lock()
				c.errText = msg
				c.mu.Unlock()
			}
		}
		if err != nil {
			closeConn("Discord 連線中斷，稍後重試")
			retry.Reset(2 * time.Second)
			continue
		}
		dirty = false
	}
}

func dial(clientID string) (io.ReadWriteCloser, string, error) {
	var conn io.ReadWriteCloser
	for i := 0; i < 10; i++ {
		cn, err := openPipe(i)
		if err == nil {
			conn = cn
			break
		}
	}
	if conn == nil {
		return nil, "", errors.New("找不到 Discord App（請確認 Discord 已開啟）")
	}
	if err := writeFrame(conn, opHandshake, map[string]any{"v": 1, "client_id": clientID}); err != nil {
		conn.Close()
		return nil, "", err
	}
	op, data, err := readFrame(conn)
	if err != nil {
		conn.Close()
		return nil, "", err
	}
	if op == opClose {
		conn.Close()
		return nil, "", fmt.Errorf("Discord 拒絕連線：%v（請檢查 Application ID）", data["message"])
	}
	user := ""
	if d, ok := data["data"].(map[string]any); ok {
		if u, ok := d["user"].(map[string]any); ok {
			if s, _ := u["global_name"].(string); s != "" {
				user = s
			} else if s, _ := u["username"].(string); s != "" {
				user = s
			}
		}
	}
	return conn, user, nil
}

// readReply skips pings until a command reply arrives.
func readReply(conn io.ReadWriteCloser) (map[string]any, error) {
	for {
		op, data, err := readFrame(conn)
		if err != nil {
			return nil, err
		}
		switch op {
		case opPing:
			if err := writeFrame(conn, opPong, data); err != nil {
				return nil, err
			}
		case opClose:
			return nil, errors.New("closed by Discord")
		case opFrame:
			return data, nil
		}
	}
}

func writeFrame(w io.Writer, op int32, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	buf := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint32(buf[0:], uint32(op))
	binary.LittleEndian.PutUint32(buf[4:], uint32(len(body)))
	copy(buf[8:], body)
	_, err = w.Write(buf)
	return err
}

func readFrame(r io.Reader) (int32, map[string]any, error) {
	head := make([]byte, 8)
	if _, err := io.ReadFull(r, head); err != nil {
		return 0, nil, err
	}
	op := int32(binary.LittleEndian.Uint32(head[0:]))
	n := binary.LittleEndian.Uint32(head[4:])
	if n > 1<<20 {
		return 0, nil, errors.New("frame too large")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	var data map[string]any
	_ = json.Unmarshal(body, &data)
	return op, data, nil
}
