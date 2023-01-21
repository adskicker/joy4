package ts

import (
	"bufio"
	"fmt"
	"time"
	"github.com/nareix/joy4/utils/bits/pio"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/format/ts/tsio"
	"github.com/nareix/joy4/codec/aacparser"
	"github.com/nareix/joy4/codec/h264parser"
	"github.com/nareix/joy4/codec"
	"io"
)

type Demuxer struct {
	r *bufio.Reader

	pkts []av.Packet

	pat     *tsio.PAT
	pmt     *tsio.PMT
	streams []*Stream
	header_to_reprocess bool
	seqnum  []int
	tshdr   []byte

	stage int
}

func NewDemuxer(r io.Reader) *Demuxer {
	return &Demuxer{
		tshdr: make([]byte, 188),
		r: bufio.NewReaderSize(r, pio.RecommendBufioSize),
	}
}

func (self *Demuxer) Streams() (streams []av.CodecData, err error) {
	if err = self.probe(); err != nil {
		return
	}
	for _, stream := range self.streams {
		streams = append(streams, stream.CodecData)
	}
	return
}

func (self *Demuxer) probe() (err error) {
	if self.stage == 0 {
		for {
			if self.pmt != nil {
				n := 0
				for _, stream := range self.streams {
					if stream.CodecData != nil {
						n++
						//fmt.Println("probe stream.CodecData != nil",id)
					}
				}
				if n == len(self.streams) {
					//fmt.Println("probe n == len(self.streams)",n)
					break
				}
			}
			if err = self.poll(); err != nil {
				return
			}
		}
		//fmt.Println("probe self.stage++")
		self.stage++

	}
	return
}

func (self *Demuxer) ReadPacket() (pkt av.Packet, err error) {
	if err = self.probe(); err != nil {
		return
	}

	for len(self.pkts) == 0 {
		if err = self.poll(); err != nil {
			return
		}
	}

	pkt = self.pkts[0]
	self.pkts = self.pkts[1:]
	return
}

func (self *Demuxer) poll() (err error) {
	if err = self.readTSPacket(); err == io.EOF {
		var n int
		if n, err = self.payloadEnd(); err != nil {
			return
		}
		if n == 0 {
			err = io.EOF
		}
	}
	return
}

func (self *Demuxer) initPMT(payload []byte) (err error) {
	var psihdrlen int
	var datalen int
	if _, _, psihdrlen, datalen, err = tsio.ParsePSI(payload); err != nil {
		return
	}
	self.pmt = &tsio.PMT{}
	//fmt.Println(psihdrlen,datalen)
	//fmt.Println(payload)
	//fmt.Println(payload[psihdrlen:psihdrlen+datalen])

	if _, err = self.pmt.Unmarshal(payload[psihdrlen:psihdrlen+datalen]); err != nil {
		return
	}
	fmt.Println(self.pmt)

	self.streams = []*Stream{}
	for i, info := range self.pmt.ElementaryStreamInfos {
		//fmt.Println(info, info.ElementaryPID,info.StreamType)
		stream := &Stream{}
		stream.idx = i
		stream.demuxer = self
		stream.pid = info.ElementaryPID
		stream.streamType = info.StreamType
		switch info.StreamType {
		case tsio.ElementaryStreamTypeH264:
			fmt.Println("ElementaryStreamTypeH264")
			self.streams = append(self.streams, stream)
			self.seqnum = append(self.seqnum,int(0))
		case tsio.ElementaryStreamTypeAdtsAAC:
			fmt.Println("ElementaryStreamTypeAdtsAAC")
			self.streams = append(self.streams, stream)
			self.seqnum = append(self.seqnum,int(0))
		case tsio.ElementaryStreamTypePrivateData:
			for _, t := range info.Descriptors {
				if t.Tag == 0x7a {
					self.streams = append(self.streams, stream)
					self.seqnum = append(self.seqnum,int(0))
					fmt.Println("ElementaryStreamTypePrivateData with EAC3")
					break
				} 
			}
		case tsio.ElementaryStreamTypeEAC3:
			self.streams = append(self.streams, stream)
			self.seqnum = append(self.seqnum,int(0))
			fmt.Println("ElementaryStreamTypeEAC3")
		}
	}
	return
}

func (self *Demuxer) payloadEnd() (n int, err error) {
	for _, stream := range self.streams {
		var i int
		if i, err = stream.payloadEnd(); err != nil {
			return
		}
		n += i
	}
	return
}

func (self *Demuxer) readTSPacket() (err error) {
	var hdrlen int
	var pid uint16
	var start bool
	var iskeyframe bool
	var seqnum int

	if !self.header_to_reprocess {
		if _, err = io.ReadFull(self.r, self.tshdr); err != nil {
			return
		}
	}
	self.header_to_reprocess = false

	if pid, start, iskeyframe, hdrlen, seqnum, err = tsio.ParseTSHeader(self.tshdr); err != nil {
		fmt.Println("ParseTSHeader error")
		return
	}
	payload := self.tshdr[hdrlen:]
	//fmt.Println(pid,start,iskeyframe,hdrlen,seqnum,payload)

	if self.pat == nil {

		if pid == 0 {

			var psihdrlen int
			var datalen int
			if _, _, psihdrlen, datalen, err = tsio.ParsePSI(payload); err != nil {
				fmt.Println("ParsePSI error")
				return
			}
			self.pat = &tsio.PAT{}
			if _, err = self.pat.Unmarshal(payload[psihdrlen:psihdrlen+datalen]); err != nil {
				fmt.Println("Unmarshal error")
				return
			}
			/*for _, entry := range self.pat.Entries {
				fmt.Println(entry)
			}*/	
		}
	} else if self.pmt == nil {
		//fmt.Println("PMT")
		for _, entry := range self.pat.Entries {
			fmt.Println(entry, pid)

			if entry.ProgramMapPID == pid {
				if err = self.initPMT(payload); err != nil {
					return
				}
				break
			}
		}
	} else {
		for id, stream := range self.streams {
			if pid == stream.pid {
				
				if start {
					var n int
					if n, err = stream.payloadEnd(); err != nil || n != 0 {
						self.header_to_reprocess = true
						return
					} 
				}
				old_seqnum := self.seqnum[id]
				self.seqnum[id] = seqnum

				if err = stream.handleTSPacket(start, iskeyframe, payload, seqnum,old_seqnum); err != nil {
					return
				}
				break
			}
		}
	}

	return
}

func (self *Stream) addPacket(payload []byte, timedelta time.Duration) {
	dts := self.dts
	pts := self.pts
	if dts == 0 {
		dts = pts
	}

	demuxer := self.demuxer
	pkt := av.Packet{
		Idx: int8(self.idx),
		IsKeyFrame: self.iskeyframe,
		Time: dts+timedelta,
		Data: payload,
	}
	if pts != dts {
		pkt.CompositionTime = pts-dts
	}
	demuxer.pkts = append(demuxer.pkts, pkt)
}

func (self *Stream) payloadEnd() (n int, err error) {
	self.isstarted = false

	payload := self.data
	if payload == nil {
		return
	}
	if self.datalen != 0 && len(payload) != self.datalen {
		err = fmt.Errorf("ts: packet size mismatch size=%d correct=%d for pid=%d",len(payload), self.datalen, self.pid)
		self.data = nil
		self.datalen = 0
		return
	}
	self.data = nil

	switch self.streamType {
	case tsio.ElementaryStreamTypeEAC3:
	case tsio.ElementaryStreamTypePrivateData:
		//var config aacparser.MPEG4AudioConfig

		delta := time.Duration(0)
		for len(payload) > 0 {
			var frame_type uint8
			var frame_size uint16
			var num_blocks uint8
			var sample_rate uint16
			var channel_count uint8
			var errParse error
			var cl av.ChannelLayout
			
			if frame_type , frame_size , num_blocks , sample_rate , channel_count , errParse = codec.ParseEAC3Header(payload);errParse != nil {
				fmt.Println("error ParseEAC3Header",errParse,len(payload),payload,self.datalen)
				err=errParse
				return
			}

			sample_count := int(num_blocks)*256


			if self.CodecData == nil {
				cl = av.CH_STEREO
				if channel_count == 1 {
					cl = av.CH_MONO
				}
				if channel_count == 3 {
					cl = av.CH_2POINT1
				}		
				// TODO manage other layout		
				self.CodecData = codec.NewEAC3CodecData(int(sample_rate),cl)
			}
			
			// RESERVED TYPE
			if frame_type != 3 {
				self.addPacket(payload[0:frame_size+2], delta)
			}
			n++

			delta += time.Duration(sample_count) * time.Second / time.Duration(sample_rate)
			payload = payload[frame_size+2:]
		}

	case tsio.ElementaryStreamTypeAdtsAAC:
		var config aacparser.MPEG4AudioConfig

		delta := time.Duration(0)
		for len(payload) > 0 {
			var hdrlen, framelen, samples int
			if config, hdrlen, framelen, samples, err = aacparser.ParseADTSHeader(payload); err != nil {
				return
			}
			if self.CodecData == nil {
				if self.CodecData, err = aacparser.NewCodecDataFromMPEG4AudioConfig(config); err != nil {
					return
				}
			}
			self.addPacket(payload[hdrlen:framelen], delta)
			n++
			delta += time.Duration(samples) * time.Second / time.Duration(config.SampleRate)
			payload = payload[framelen:]
		}

	case tsio.ElementaryStreamTypeH264:
		nalus, _ := h264parser.SplitNALUs(payload)
		var sps, pps []byte
		for _, nalu := range nalus {
			if len(nalu) > 0 {
				naltype := nalu[0] & 0x1f
				switch {
				case naltype == 7:
					sps = nalu
				case naltype == 8:
					pps = nalu
				case h264parser.IsDataNALU(nalu):
					// raw nalu to avcc
					b := make([]byte, 4+len(nalu))
					pio.PutU32BE(b[0:4], uint32(len(nalu)))
					copy(b[4:], nalu)
					self.addPacket(b, time.Duration(0))
					n++
				}
			}
		}

		if self.CodecData == nil && len(sps) > 0 && len(pps) > 0 {
			if self.CodecData, err = h264parser.NewCodecDataFromSPSAndPPS(sps, pps); err != nil {
				return
			}
		}
	}

	return
}

func (self *Stream) handleTSPacket(start bool, iskeyframe bool, payload []byte, seqnum int, old_seqnum int) (err error) {
	if start {
		/*if _, err = self.payloadEnd(); err != nil {
			return
		}*/
		var hdrlen int
		if hdrlen, _, self.datalen, self.pts, self.dts, err = tsio.ParsePESHeader(payload); err != nil {
			return
		}
		self.iskeyframe = iskeyframe
		if self.datalen == 0 {
			self.data = make([]byte, 0, 4096)
		} else {
			self.data = make([]byte, 0, self.datalen)
		}
		self.data = append(self.data, payload[hdrlen:]...)
		self.isstarted = true
	} else {
		if self.isstarted {
			if (old_seqnum + 1)%16 != seqnum {
				fmt.Println("ERROR seqnum non contiguous:",self.pid,old_seqnum,seqnum)
				//self.datalen = 0
			}
			self.data = append(self.data, payload...)
		}
	}
	return
}
