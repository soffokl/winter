package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/xid"
)

var (
	errWrongShootingCmd = fmt.Errorf("wrong shoot command")
	errShootMissed      = fmt.Errorf("shoot missed the targed")
	errByteToInt        = fmt.Errorf("failed to convert []byte to int")
)

const (
	lobbyStartMsg   = "New game is started. ID:"
	lobbyHelpMsg    = "Use 'start {player} [ID]' command to start a new game. ID is optional, use it to join existing game."
	lobbyUnknownCmd = "Unknown lobby command, use 'help' to get list of available commands"
)

type game struct {
	id     xid.ID
	ch     chan dto
	ctx    context.Context
	w, h   int
	active bool

	z       zombie
	lock    sync.RWMutex
	players map[xid.ID]*player
}

type zombie struct {
	name string
	x, y int
}

type player struct {
	name string
	rwc  io.ReadWriter
}

type dto struct {
	data []byte
	name string
}

var pool = new(sync.Map)

func newSession(rwc io.ReadWriteCloser, h, w int, sleep time.Duration) {
	defer rwc.Close()

	g := newGame(rwc, w, h)
	for g.lobby() {
		select {
		case <-g.ctx.Done():
			return
		default:
			g.gameplay(sleep)
		}
	}
}

func newGame(rwc io.ReadWriter, w, h int) *game {
	ctx, cancel := context.WithCancel(context.Background())

	g := &game{
		ctx: ctx,
		ch:  make(chan dto),
		players: map[xid.ID]*player{
			xid.New(): &player{
				name: "Jon Snow",
				rwc:  rwc,
			},
		},
		w: w,
		h: h,
	}

	go func() {
		defer cancel()
		defer close(g.ch)
		g.read()
	}()

	return g
}

func (g *game) lobby() (newGame bool) {
	g.id = xid.New()
	g.z = zombie{y: rand.Intn(g.h), name: "night-king"}

	for cmd := range g.ch {
		args := bytes.Fields(cmd.data)
		if len(args) == 0 {
			continue
		}

		switch string(args[0]) {
		case "start":
			g.active = true
			if len(args) < 2 {
				g.broadcast(lobbyHelpMsg)
				break
			}

			_, p := g.initiator()
			g.lock.Lock()
			p.name = string(args[1])
			g.lock.Unlock()

			if len(args) > 2 {
				if newID, err := xid.FromString(string(args[2])); err == nil {
					g.id = newID
				}
			}

			if active, ok := pool.Load(g.id); ok {
				if g.join(active.(*game)) {
					continue
				}
			}

			pool.Store(g.id, g)
			g.broadcast(lobbyStartMsg, g.id)

			return true

		case "help":
			g.broadcast(lobbyHelpMsg)

		default:
			g.broadcast(lobbyUnknownCmd)
		}
	}

	return true
}

func (g *game) gameplay(sleep time.Duration) {
	t := time.NewTicker(sleep)
	defer t.Stop()

	for !g.isDone() {
		select {
		case <-g.ctx.Done():
			return

		case cmd := <-g.ch:
			if err := g.shoot(cmd.data); err != nil {
				switch err {
				case errShootMissed:
					g.broadcast("BOOM", cmd.name, 0)
				default:
					g.broadcast(err)
				}
				break
			}
			g.done()
			g.broadcast("BOOM", cmd.name, "1", g.z.name)

		case <-t.C:
			if g.z.move(g.w, g.h) {
				g.done()
				g.broadcast("BOOM", g.z.name, "reached the wall")
				break
			}
			g.broadcast("WALK", g.z.name, g.z.x, g.z.y)
		}
	}
}

func (g *game) join(active *game) bool {
	if active == nil {
		return false
	}
	id, p := g.initiator()
	active.lock.Lock()
	active.players[id] = p
	active.lock.Unlock()

	defer func() {
		active.lock.Lock()
		delete(active.players, id)
		active.lock.Unlock()
	}()

	// for _, ok := pool.Load(g.id); ok; _, ok = pool.Load(g.id) {
	for msg := range g.ch {
		active.ch <- msg
		if active.isDone() {
			return true
		}
	}
	// }

	return true
}

func (g *game) broadcast(a ...interface{}) {
	g.lock.RLock()
	defer g.lock.RUnlock()

	for _, p := range g.players {
		fmt.Fprintln(p.rwc, a...)
	}
}

func (g *game) initiator() (id xid.ID, p *player) {
	g.lock.RLock()
	defer g.lock.RUnlock()

	for id, p := range g.players {
		return id, p
	}

	return xid.ID{}, nil
}

func (g *game) read() {
	_, p := g.initiator()
	buf := bufio.NewReader(p.rwc)
	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			return
		}
		g.lock.RLock()
		name := p.name
		g.lock.RUnlock()
		g.ch <- dto{data: sliceCopy(line), name: name}
	}
}

func (g *game) shoot(command []byte) error {
	args := bytes.Fields(bytes.ToLower(command))
	if len(args) < 3 || string(args[0]) != "shoot" {
		return errWrongShootingCmd
	}

	x, err := btoi(args[1])
	if err != nil {
		return err
	}
	y, err := btoi(args[2])
	if err != nil {
		return err
	}

	return g.z.kill(x, y)
}

func (g *game) isDone() bool {
	g.lock.RLock()
	defer g.lock.RUnlock()

	return !g.active
}

func (g *game) done() {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.active = false
	pool.Delete(g.id)
}

func (z *zombie) kill(x, y int) error {
	if z.x == x && z.y == y {
		return nil
	}
	return errShootMissed
}

func (z *zombie) move(xLimit, yLimit int) bool {
	z.x, z.y = (z.x + rand.Intn(2)), (z.y + rand.Intn(3) - 1)

	if z.y < 0 {
		z.y = 0
	}
	if z.y >= yLimit {
		z.y = yLimit - 1
	}

	return z.x >= xLimit
}
