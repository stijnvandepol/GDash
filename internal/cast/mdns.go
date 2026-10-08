package cast

import (
	"net"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

func buildQuery() ([]byte, error) {
	name, err := dnsmessage.NewName(castService)
	if err != nil {
		return nil, err
	}
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{})
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	err = b.Question(dnsmessage.Question{
		Name: name,
		Type: dnsmessage.TypePTR,
		// The top bit asks for a unicast response (the "QU" bit).
		Class: dnsmessage.ClassINET | 1<<15,
	})
	if err != nil {
		return nil, err
	}
	return b.Finish()
}

type serviceInstance struct {
	host string
	port uint16
	txt  map[string]string
	from net.IP
}

// collector merges mDNS answers, which can spread one device over several
// records and packets.
type collector struct {
	instances map[string]*serviceInstance
	hosts     map[string]net.IP
}

func newCollector() *collector {
	return &collector{instances: map[string]*serviceInstance{}, hosts: map[string]net.IP{}}
}

func (c *collector) instance(name string) *serviceInstance {
	inst, ok := c.instances[name]
	if !ok {
		inst = &serviceInstance{txt: map[string]string{}}
		c.instances[name] = inst
	}
	return inst
}

func (c *collector) add(packet []byte, from net.IP) error {
	var p dnsmessage.Parser
	if _, err := p.Start(packet); err != nil {
		return err
	}
	if err := p.SkipAllQuestions(); err != nil {
		return err
	}
	var records []dnsmessage.Resource
	answers, err := p.AllAnswers()
	if err != nil {
		return err
	}
	records = append(records, answers...)
	if err := p.SkipAllAuthorities(); err != nil {
		return err
	}
	additionals, err := p.AllAdditionals()
	if err != nil {
		return err
	}
	records = append(records, additionals...)

	for _, r := range records {
		c.record(r, from)
	}
	return nil
}

// record merges one resource record into what is known so far.
func (c *collector) record(r dnsmessage.Resource, from net.IP) {
	name := strings.ToLower(r.Header.Name.String())
	switch body := r.Body.(type) {
	case *dnsmessage.PTRResource:
		if name == castService {
			c.instance(strings.ToLower(body.PTR.String())).from = from
		}
	case *dnsmessage.SRVResource:
		inst := c.instance(name)
		inst.host = strings.ToLower(body.Target.String())
		inst.port = body.Port
		inst.from = from
	case *dnsmessage.TXTResource:
		addTXT(c.instance(name), body.TXT)
	case *dnsmessage.AResource:
		c.hosts[name] = net.IP(body.A[:])
	}
}

func addTXT(inst *serviceInstance, entries []string) {
	for _, kv := range entries {
		if key, value, ok := strings.Cut(kv, "="); ok {
			inst.txt[key] = value
		}
	}
}

func (c *collector) devices() []Device {
	byID := map[string]Device{}
	for _, inst := range c.instances {
		id := inst.txt["id"]
		if id == "" || inst.port == 0 {
			continue
		}
		ip := c.hosts[inst.host]
		if ip == nil {
			ip = inst.from
		}
		if ip == nil {
			continue
		}
		byID[id] = Device{
			ID:        id,
			Name:      inst.txt["fn"],
			Model:     inst.txt["md"],
			Addr:      net.JoinHostPort(ip.String(), strconv.Itoa(int(inst.port))),
			HasScreen: hasVideoOut(inst.txt["ca"]),
		}
	}
	devices := make([]Device, 0, len(byID))
	for _, d := range byID {
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices
}
