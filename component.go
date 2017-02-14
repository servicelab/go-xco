package xco

import (
	"encoding/xml"
	"fmt"
	"log"
	"net"

	"github.com/pkg/errors"

	"golang.org/x/net/context"
)

type stateFn func() (stateFn, error)

// A Component is an instance of a Jabber Component (XEP-0114)
type Component struct {
	MessageHandler   MessageHandler
	DiscoInfoHandler DiscoInfoHandler
	PresenceHandler  PresenceHandler
	IqHandler        IqHandler
	UnknownHandler   UnknownElementHandler

	ctx      context.Context
	cancelFn context.CancelFunc

	conn net.Conn
	dec  *xml.Decoder
	enc  *xml.Encoder
	log  *log.Logger

	stateFn stateFn

	sharedSecret string
	name         string

	// channels for XMPP stanzas
	tx   <-chan interface{} // outgoing stanzas
	rx   chan<- interface{} // incoming stanzas
	errx chan<- error       // errors
}

func (c *Component) init(o Options) error {
	conn, err := net.Dial("tcp", o.Address)
	if err != nil {
		return err
	}

	c.MessageHandler = noOpMessageHandler
	c.DiscoInfoHandler = noOpDiscoInfoHandler
	c.PresenceHandler = noOpPresenceHandler
	c.IqHandler = noOpIqHandler
	c.UnknownHandler = noOpUnknownHandler

	c.conn = conn
	c.name = o.Name
	c.sharedSecret = o.SharedSecret
	if o.Logger == nil {
		c.dec = xml.NewDecoder(conn)
		c.enc = xml.NewEncoder(conn)
	} else {
		c.log = o.Logger
		c.dec = xml.NewDecoder(newReadLogger(c.log, conn))
		c.enc = xml.NewEncoder(newWriteLogger(c.log, conn))
	}
	c.stateFn = c.handshakeState

	return nil
}

// Close closes the Component
func (c *Component) Close() {
	if c == nil {
		return
	}
	c.cancelFn()
}

// Run runs the component handlers loop and waits for it to finish.
// This is a convenience wrapper around RunAsync to make it easier to
// write synchronous components.
func (c *Component) Run() (err error) {
	tx, rx, errx := c.RunAsync()
	for {
		var err error
		select {
		case stanza := <-rx:
			switch x := stanza.(type) {
			case *Message:
				err = c.MessageHandler(c, x)
			case *Presence:
				err = c.PresenceHandler(c, x)
			case *Iq:
				if x.IsDiscoInfo() {
					x, err = c.discoInfo(x)
					if err == nil {
						tx <- x
					}
				} else {
					err = c.IqHandler(c, x)
				}
			case *xml.StartElement:
				err = c.UnknownHandler(c, x)
			default:
				panic(fmt.Sprintf("Unexpected stanza type: %#v", stanza))
			}
		case err = <-errx:
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// RunAsync runs the component asynchronously and returns channels for
// sending and receiving XML stanzas.  If an error occurs, the error
// value is sent down the error channel and then both errx and rx
// channels are closed.  If you close the tx channel, the entire component
// gracefully shuts down.
//
// Usage is something like this:
//
//     tx, rx, errx := c.RunAsync()
//     for {
//         stanza, ok := <-rx  // wait for a stanza to arrive
//         if !ok {
//             break
//         }
//         tx <- &xco.Message{Body: "Hi"}
//     }
func (c *Component) RunAsync() (chan<- interface{}, <-chan interface{}, <-chan error) {
	tx := make(chan interface{})
	rx := make(chan interface{})
	errx := make(chan error)

	c.tx = tx
	c.rx = rx
	c.errx = errx

	go c.runReadLoop()
	go c.runWriteLoop()

	return tx, rx, errx
}

// handle writing to the XMPP server connection
func (c *Component) runWriteLoop() {
	for {
		select {
		case _ = <-c.ctx.Done():
			return
		case stanza, ok := <-c.tx:
			if !ok {
				c.cancelFn()
				return
			}
			if err := c.Send(stanza); err != nil {
				c.errx <- err
				c.cancelFn()
				return
			}
		}
	}
}

// handle reading from the XMPP server connection
func (c *Component) runReadLoop() {
	defer func() {
		close(c.rx)
		close(c.errx)
		c.conn.Close()
	}()

	var err error
LOOP:
	for {
		select {
		case _ = <-c.ctx.Done():
			break LOOP
		default:
			if c.stateFn == nil {
				break LOOP
			}
			c.stateFn, err = c.stateFn()
			if err != nil {
				break LOOP
			}
		}
	}

	c.errx <- err
	c.cancelFn()
}

// Send sends the given pointer struct by serializing it to XML.
func (c *Component) Send(i interface{}) error {
	return errors.Wrap(c.enc.Encode(i), "Error encoding object to XML")
}

// Write implements the io.Writer interface to allow direct writing to the XMPP connection
func (c *Component) Write(b []byte) (int, error) {
	return c.conn.Write(b)
}
