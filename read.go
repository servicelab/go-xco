package xco

import "encoding/xml"

// UnknownElementHandler handles unknown XML entities sent through XMPP
type UnknownElementHandler func(*Component, *xml.StartElement) error

func noOpUnknownHandler(c *Component, x *xml.StartElement) error {
	return nil
}

func (c *Component) readLoopState() (stateFn, error) {

	t, err := c.dec.Token()
	if err != nil {
		return nil, err
	}

	if st, ok := t.(xml.StartElement); ok {

		if st.Name.Local == "message" {
			var m Message
			if err := c.dec.DecodeElement(&m, &st); err != nil {
				return nil, err
			}
			c.rx <- &m
		} else if st.Name.Local == "presence" {
			var p Presence
			if err := c.dec.DecodeElement(&p, &st); err != nil {
				return nil, err
			}
			c.rx <- &p
		} else if st.Name.Local == "iq" {
			var iq Iq
			if err := c.dec.DecodeElement(&iq, &st); err != nil {
				return nil, err
			}
			c.rx <- &iq
		} else {
			c.rx <- &st
		}
	}

	return c.readLoopState, nil
}

// DiscoInfoReply returns a new Iq stanza which is a reply to the
// given service discovery info stanza.  It only makes sense to call
// this method if IsDiscoInfo returns true.
func (iq *Iq) DiscoInfoReply(ids []DiscoIdentity, features []DiscoFeature) (*Iq, error) {
	if len(ids) < 1 {
		return nil, nil
	}

	features = append(features, DiscoFeature{
		Var: discoInfoSpace,
	})
	query := DiscoInfoQuery{
		Identities: ids,
		Features:   features,
	}
	queryContent, err := xml.Marshal(query)
	if err != nil {
		return nil, err
	}
	resp := &Iq{
		Header: Header{
			From: iq.To,
			To:   iq.From,
			ID:   iq.ID,
		},
		Type:    "result",
		Content: string(queryContent),
		XMLName: iq.XMLName,
	}
	return resp, nil
}
