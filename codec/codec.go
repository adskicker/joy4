package codec

import (
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/fake"
	"github.com/nareix/joy4/utils/bits/pio"
	"time"
	"fmt"
)

type PCMUCodecData struct {
	typ av.CodecType
}

func (self PCMUCodecData) Type() av.CodecType {
	return self.typ
}

func (self PCMUCodecData) SampleRate() int {
	return 8000
}

func (self PCMUCodecData) ChannelLayout() av.ChannelLayout {
	return av.CH_MONO
}

func (self PCMUCodecData) SampleFormat() av.SampleFormat {
	return av.S16
}

func (self PCMUCodecData) PacketDuration(data []byte) (time.Duration, error) {
	return time.Duration(len(data)) * time.Second / time.Duration(8000), nil
}

func NewPCMMulawCodecData() av.AudioCodecData {
	return PCMUCodecData{
		typ: av.PCM_MULAW,
	}
}

func NewPCMAlawCodecData() av.AudioCodecData {
	return PCMUCodecData{
		typ: av.PCM_ALAW,
	}
}

type SpeexCodecData struct {
	fake.CodecData
}

func (self SpeexCodecData) PacketDuration(data []byte) (time.Duration, error) {
	// libavcodec/libspeexdec.c
	// samples = samplerate/50
	// duration = 0.02s
	return time.Millisecond*20, nil
}

func NewSpeexCodecData(sr int, cl av.ChannelLayout) SpeexCodecData {
	codec := SpeexCodecData{}
	codec.CodecType_ = av.SPEEX
	codec.SampleFormat_ = av.S16
	codec.SampleRate_ = sr
	codec.ChannelLayout_ = cl
	return codec
}

/*
const uint16_t ff_ac3_channel_layout_tab[8] = {
    AV_CH_LAYOUT_STEREO,
    AV_CH_LAYOUT_MONO,
    AV_CH_LAYOUT_STEREO,
    AV_CH_LAYOUT_SURROUND,
    AV_CH_LAYOUT_2_1,
    AV_CH_LAYOUT_4POINT0,
    AV_CH_LAYOUT_2_2,
    AV_CH_LAYOUT_5POINT0
};

const int ff_ac3_sample_rate_tab[] = { 48000, 44100, 32000, 0 };


        hdr->sr_code = get_bits(gbc, 2);
        if (hdr->sr_code == 3) {
            int sr_code2 = get_bits(gbc, 2);
            if(sr_code2 == 3)
                return AAC_AC3_PARSE_ERROR_SAMPLE_RATE;
            hdr->sample_rate = ff_ac3_sample_rate_tab[sr_code2] / 2;
            hdr->sr_shift = 1;
        } else {
            hdr->num_blocks = eac3_blocks[get_bits(gbc, 2)];
            hdr->sample_rate = ff_ac3_sample_rate_tab[hdr->sr_code];
            hdr->sr_shift = 0;
        }

*/
/*EAC3_FRAME_TYPE_INDEPENDENT = 0,
EAC3_FRAME_TYPE_DEPENDENT,
EAC3_FRAME_TYPE_AC3_CONVERT,
EAC3_FRAME_TYPE_RESERVED*/
//hdr->channels = ff_ac3_channels_tab[hdr->channel_mode] + hdr->lfe_on;


var ErrParseEAC3Header = fmt.Errorf("invalid EAC3 header")
var ErrParseEAC3SyncWord = fmt.Errorf("invalid EAC3 header")
var ErrParseEAC3Frame = fmt.Errorf("invalid EAC3 frame")

var ac3_sample_rate = []uint16{48000, 44100, 32000, 0}
var eac3_blocks = [4]uint8{1, 2, 3, 6}
var ac3_channels = [8]uint8{2, 1, 2, 3, 3, 4, 4, 5}

func ParseEAC3Header(payload []byte) (frame_type uint8, frame_size uint16, num_blocks uint8, sample_rate uint16, channel_count uint8, err error) {

	num_blocks = 6

	if len(payload) < 7 {
		err = ErrParseEAC3Header
		return
	}
	var syncword uint16	= pio.U16BE(payload[0:2])

	if syncword != 0x0b77 {
		err = ErrParseEAC3SyncWord
		return
	}

	//get frame type
	frame_type = ((payload[2])&0xc0)>>6


	// does not include the sync word
	frame_size = (pio.U16BE(payload[2:2+2])&0x7ff)*2
	
	if len(payload) < int(frame_size+2) {
		err = ErrParseEAC3Frame
		return
	}
	
	//get sr code
	sr_code := (payload[4]&0xc0)>>6
	if sr_code == 3 {
		sr_code = (payload[4]&0x30)>>4
		sample_rate = ac3_sample_rate[sr_code]/2
	} else {
		num_blocks = eac3_blocks[(payload[4]&0x30)>>4]
		sample_rate = ac3_sample_rate[sr_code]
	}

	channel_mode := (payload[4]&0x0e)>>1
	lfe_on := (payload[4]&0x01)

	channel_count = ac3_channels[channel_mode] + lfe_on

	//fmt.Println(frame_type,frame_size,num_blocks,sample_rate,channel_count,payload[0:6])

	return
}

type EAC3CodecData struct {
	fake.CodecData
}

func (self EAC3CodecData) PacketDuration(data []byte) (time.Duration, error) {
	return time.Second/time.Duration(self.SampleRate_), nil
}

func NewEAC3CodecData(sr int, cl av.ChannelLayout) EAC3CodecData {
	codec := EAC3CodecData{}
	codec.CodecType_ = av.EAC3
	codec.SampleFormat_ = av.FLTP
	codec.SampleRate_ = sr
	codec.ChannelLayout_ = cl
	return codec
}



