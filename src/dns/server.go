package dns

import (
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var recordsA map[string]string = make(map[string]string)
var recordsAAAA map[string]string = make(map[string]string)

const upstreamDNSHost = "1.1.1.1:53"

func find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func toDNSPacket(data []byte) *layers.DNS {
	packet := gopacket.NewPacket(data, layers.LayerTypeDNS, gopacket.Default)
	dnsLayer := packet.Layer(layers.LayerTypeDNS)
	dnsPacket, _ := dnsLayer.(*layers.DNS)
	return dnsPacket
}

func createDNSAnswerA(name, ip string) layers.DNSResourceRecord {
	var dnsAnswer layers.DNSResourceRecord
	dnsAnswer.Type = layers.DNSTypeA
	dnsAnswer.Name = []byte(name)
	dnsAnswer.Class = layers.DNSClassIN

	a, _, _ := net.ParseCIDR(ip + "/24")
	dnsAnswer.IP = a
	return dnsAnswer
}

func createDNSAnswerAAAA(name, ip string) layers.DNSResourceRecord {
	var dnsAnswer layers.DNSResourceRecord
	dnsAnswer.Type = layers.DNSTypeAAAA
	dnsAnswer.Name = []byte(name)
	dnsAnswer.Class = layers.DNSClassIN

	a, _, _ := net.ParseCIDR(ip + "/32")
	dnsAnswer.IP = a
	return dnsAnswer
}

// Server for DNS requests
func Server(blocklist []string) {
	addr := net.UDPAddr{
		Port: 8090,
		IP:   net.ParseIP("0.0.0.0"),
	}
	u, _ := net.ListenUDP("udp", &addr)

	for {
		var data []byte
		var clientAddr net.Addr
		{
			tmp := make([]byte, 1024)
			n, addr, _ := u.ReadFrom(tmp)
			data = tmp[:n]
			clientAddr = addr
		}
		dnsPacket := toDNSPacket(data)
		question := dnsPacket.Questions[0]
		requestType := question.Type
		name := string(question.Name)
		_, block := find(blocklist, name)
		var cache map[string]string
		var packetGenFunction func(a, b string) layers.DNSResourceRecord

		switch requestType {
		case layers.DNSTypeA:
			cache = recordsA
			packetGenFunction = createDNSAnswerA
		case layers.DNSTypeAAAA:
			cache = recordsAAAA
			packetGenFunction = createDNSAnswerAAAA
		default:
			continue
		}

		if block {
			dnsPacket.Answers = nil
			dnsPacket.ANCount = 0

			dnsPacket.QR = true
			dnsPacket.OpCode = layers.DNSOpCodeNotify
			dnsPacket.AA = true
			dnsPacket.ResponseCode = layers.DNSResponseCodeNoErr

			buf := gopacket.NewSerializeBuffer()
			_ = dnsPacket.SerializeTo(buf, gopacket.SerializeOptions{})
			u.WriteTo(buf.Bytes(), clientAddr)
			continue
		}

		ip, exists := cache[name]
		if exists {
			// handle non-existing server as an empty string
			if len(ip) > 0 {
				answer := packetGenFunction(name, ip)
				dnsPacket.Answers = append(dnsPacket.Answers, answer)
				dnsPacket.ANCount = 1
			} else {
				dnsPacket.Answers = nil
				dnsPacket.ANCount = 0
			}
			dnsPacket.QR = true
			dnsPacket.OpCode = layers.DNSOpCodeNotify
			dnsPacket.AA = true
			dnsPacket.ResponseCode = layers.DNSResponseCodeNoErr

			buf := gopacket.NewSerializeBuffer()
			_ = dnsPacket.SerializeTo(buf, gopacket.SerializeOptions{})
			u.WriteTo(buf.Bytes(), clientAddr)
		} else {
			var dnsResponse []byte
			{
				upstreamConn, _ := net.Dial("udp", upstreamDNSHost)
				defer upstreamConn.Close()
				upstreamConn.Write(data)
				tmp := make([]byte, 1024)
				udpConn, _ := upstreamConn.(*net.UDPConn)
				upstreamConn.SetReadDeadline(time.Now().Add(time.Second * 1))
				n, _, err := udpConn.ReadFrom(tmp)
				if err != nil {
					println(err)
					panic(err)
				}

				dnsResponse = tmp[:n]
			}
			dnsResponsePacket := toDNSPacket(dnsResponse)
			answers := dnsResponsePacket.Answers
			if len(answers) > 0 {
				cache[name] = answers[0].IP.String()
			} else {
				// Store a non-existing server as an empty string
				cache[name] = ""
			}
			u.WriteTo(dnsResponse, clientAddr)
		}

	}

}