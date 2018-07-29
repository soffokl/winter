package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/xid"
)

func Test_lobbyStart(t *testing.T) {
	r, w := strings.NewReader("start test"), new(bytes.Buffer)
	rw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	g := newGame(rw, 30, 10)
	g.lobby()
	rw.Flush()

	if w.String() != fmt.Sprintln(lobbyStartMsg, g.id) {
		t.Errorf("unexpected greating text: %v", w.String())
	}
}

func Test_lobbyHelp(t *testing.T) {
	r, w := strings.NewReader("help"), &bytes.Buffer{}
	rw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	g := newGame(rw, 30, 10)
	g.lobby()
	rw.Flush()

	if w.String() != fmt.Sprintln(lobbyHelpMsg) {
		t.Errorf("unexpected greating text: %v", w.String())
	}
}

func Test_lobbyUnknown(t *testing.T) {
	r, w := strings.NewReader("qwer"), &bytes.Buffer{}
	rw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	g := newGame(rw, 30, 10)
	g.lobby()
	rw.Flush()

	if w.String() != fmt.Sprintln(lobbyUnknownCmd) {
		t.Errorf("unexpected greating text: %v", w.String())
	}
}

func Test_lobbyStartWithID(t *testing.T) {
	id := xid.New().String()

	r, w := strings.NewReader("start test "+id+"\n"), &bytes.Buffer{}
	rw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	g := newGame(rw, 30, 10)
	g.lobby()
	rw.Flush()

	if id, _ := xid.FromString(id); g.id != id {
		t.Errorf("unexpected game id: %v", g.id)
	}
}

func Test_game_shoot(t *testing.T) {
	r, w := strings.NewReader("start test\nshoot 0 0"), &bytes.Buffer{}
	rw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	g := newGame(rw, 30, 10)
	g.lobby()
	rw.Flush()
	g.z.x, g.z.y = 0, 0

	for cmd := range g.ch {
		if err := g.shoot(cmd.data); err != nil {
			t.Errorf("unexpected error for valid shooting command: %v", err)
		}
	}
}

func Test_MultiUserSession(t *testing.T) {
	id := xid.New().String()

	pr0, pw0 := io.Pipe()
	pr1, pw1 := io.Pipe()
	w0, w1 := &bytes.Buffer{}, &bytes.Buffer{}

	defer pr0.Close()
	defer pw0.Close()

	go pw0.Write([]byte("start test0 " + id + "\n"))
	go pw1.Write([]byte("start test1 " + id + "\n"))

	rw0 := bufio.NewReadWriter(bufio.NewReader(pr0), bufio.NewWriter(w0))
	rw1 := bufio.NewReadWriter(bufio.NewReader(pr1), bufio.NewWriter(w1))

	g0 := newGame(rw0, 30, 10)
	g1 := newGame(rw1, 30, 10)

	g0.lobby()
	g0.z.x, g0.z.y = 0, 0
	go g0.gameplay(time.Second * 10)

	go func() {
		pw0.Write([]byte("shoot 1 1"))
		pw1.Write([]byte("shoot 0 0"))

		pw1.Close()
		pr1.Close()
	}()
	g1.lobby()

	if strings.HasSuffix(w0.String(), "BOOM test1 1 night-king") && strings.HasSuffix(w1.String(), "BOOM test2 1 night-king") {
		t.Errorf("unexpected game result: \n%v \n%v", w0.String(), w1.String())
	}
}

func Test_game_shootMiss(t *testing.T) {
	r, w := strings.NewReader("start test\nshoot 0 1"), &bytes.Buffer{}
	rw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	g := newGame(rw, 30, 10)
	g.lobby()
	rw.Flush()
	g.z.x, g.z.y = 0, 0

	for cmd := range g.ch {
		if err := g.shoot(cmd.data); err != errShootMissed {
			t.Errorf("unexpected error for missed shooting command: %v", err)
		}
	}
}

func Test_game_shootUnknownCmd(t *testing.T) {
	r, w := strings.NewReader("start test\nsho0ot 0 2"), &bytes.Buffer{}
	rw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(w))

	g := newGame(rw, 30, 10)
	g.lobby()
	rw.Flush()

	for cmd := range g.ch {
		if err := g.shoot(cmd.data); err != errWrongShootingCmd {
			t.Errorf("unexpected error for invalid shooting command: %v", err)
		}
	}
}

func Test_newSessionWin(t *testing.T) {
	r := newFakeSession(t, time.Millisecond, "", "BOOM test 1 night-king\n")

	newSession(r, 10, 30, time.Second)
}

func Test_newSessionFail(t *testing.T) {
	r := newFakeSession(t, time.Second, "", "BOOM night-king reached the wall\n")

	newSession(r, 10, 30, time.Nanosecond)
}

func Test_newSessionWinParallel(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			r := newFakeSession(t, time.Millisecond, "", "BOOM test 1 night-king\n")
			newSession(r, 10, 30, time.Second)
		}()
	}
	wg.Wait()
}

func Test_newSessionFailParallel(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			r := newFakeSession(t, time.Second, "", "BOOM night-king reached the wall\n")
			newSession(r, 10, 30, time.Nanosecond)
		}()
	}
	wg.Wait()
}

type fakeSession struct {
	input    io.ReadCloser
	output   io.WriteCloser
	t        *testing.T
	expected string
}

func newFakeSession(t *testing.T, d time.Duration, id, expected string) fakeSession {
	pr, pw := io.Pipe()
	go func() {
		fmt.Fprintln(pw, "start test"+id)
		for {
			for x := 0; x < 30; x++ {
				for y := 0; y < 10; y++ {
					time.Sleep(d)
					if _, err := fmt.Fprintln(pw, "shoot", x, y); err != nil {
						return
					}
				}
			}
		}
	}()
	return fakeSession{input: pr, output: pw, t: t, expected: expected}
}

func (f fakeSession) Read(p []byte) (int, error) {
	return f.input.Read(p)
}

func (f fakeSession) Write(b []byte) (int, error) {
	if string(b) == f.expected {
		f.input.Close()
		f.output.Close()
	}

	return len(b), nil
}

func (f fakeSession) Close() error {
	return nil
}
