package xco

// Iq represents an info/query message
type Iq struct {
	Header

	Type string `xml:"type,attr"`

	DiscoInfo *DiscoInfoQuery `xml:"http://jabber.org/protocol/disco#info query,omitempty"`

	Vcard *Vcard `xml:"vcard-temp vCard,omitempty"`

	XMLName string `xml:"iq"`
}

// IqHandler handles an incoming Iq (info/query) request
type IqHandler func(c *Component, iq *Iq) error

func noOpIqHandler(c *Component, iq *Iq) error {
	return nil
}
