package ffmpeg

/*
#include "ffmpeg.h"
int wrap_avcodec_decode_video2(AVCodecContext *ctx, AVFrame *frame, void *data, int size, int64_t dts, int64_t pts, int *got, int64_t *fpts) {
	struct AVPacket pkt;
	memset(&pkt,0,sizeof(pkt));
	
	pkt.data = data;
	pkt.size = size;
	pkt.pos = -1;
	pkt.pts = pts;
	pkt.dts = dts;

	
	//= {.data = data, .size = size, .pts = pts, .dts = dts};
//	int ret = avcodec_decode_video2(ctx, frame, got, &pkt);
//	*fpts = av_frame_get_best_effort_timestamp(frame);

    *got = 0;

	int ret = avcodec_send_packet(ctx, &pkt);

	if (ret < 0) {
		return ret;
	}

	ret = avcodec_receive_frame(ctx, frame);
	if (ret == 0) {
		 *got = 1;
	}
	return ret; 
}

int wrap_av_filter(AVFilterContext *buffersrc_ctx,AVFilterContext *buffersink_ctx, AVFrame *frame_in,AVFrame *frame_out) {

	if (av_buffersrc_add_frame_flags(buffersrc_ctx, frame_in, AV_BUFFERSRC_FLAG_KEEP_REF) < 0) {
		return -1;
	}
	
	int ret = av_buffersink_get_frame(buffersink_ctx, frame_out);

	return ret;
}


static int init_filters(AVCodecContext *dec_ctx,AVFilterGraph **filter_graph, AVFilterContext **buffersrc_ctx,AVFilterContext **buffersink_ctx)
{
	const char *filters_descr = "yadif=0:-1:0";//"yadif=0:-1:0,format=rgb24";//"yadif=1,format=yuv420p";
    char args[512];
    int ret = 0;
    const AVFilter *buffersrc  = avfilter_get_by_name("buffer");
    const AVFilter *buffersink = avfilter_get_by_name("buffersink");
    AVFilterInOut *outputs = avfilter_inout_alloc();
    AVFilterInOut *inputs  = avfilter_inout_alloc();
    //AVRational time_base = fmt_ctx->streams[video_stream_index]->time_base;
    enum AVPixelFormat pix_fmts[] = { AV_PIX_FMT_YUV420P, AV_PIX_FMT_NONE };

    *filter_graph = avfilter_graph_alloc();
    if (!outputs || !inputs || !*filter_graph) {
        ret = AVERROR(ENOMEM);
        goto end;
    }

    // buffer video source: the decoded frames from the decoder will be inserted here.
    //snprintf(args, sizeof(args),
    //        "video_size=%dx%d:pix_fmt=%d:time_base=%d/%d:pixel_aspect=%d/%d",
    //        dec_ctx->width, dec_ctx->height, dec_ctx->pix_fmt,
    //        1,90000,//time_base.num, time_base.den,
    //        dec_ctx->sample_aspect_ratio.num, dec_ctx->sample_aspect_ratio.den);
    
    snprintf(args, sizeof(args),"video_size=1920x1080:pix_fmt=0:time_base=1/90000:pixel_aspect=1/1");
    printf("%s\n",args);

    ret = avfilter_graph_create_filter(buffersrc_ctx, buffersrc, "in",
                                       args, NULL, *filter_graph);
    if (ret < 0) {
        av_log(NULL, AV_LOG_ERROR, "Cannot create buffer source\n");
        goto end;
    }

    // buffer video sink: to terminate the filter chain.
    ret = avfilter_graph_create_filter(buffersink_ctx, buffersink, "out",
                                       NULL, NULL, *filter_graph);
    if (ret < 0) {
        av_log(NULL, AV_LOG_ERROR, "Cannot create buffer sink\n");
        goto end;
    }

    ret = av_opt_set_int_list(*buffersink_ctx, "pix_fmts", pix_fmts,
                              AV_PIX_FMT_NONE, AV_OPT_SEARCH_CHILDREN);
    if (ret < 0) {
        av_log(NULL, AV_LOG_ERROR, "Cannot set output pixel format\n");
        goto end;
    }

    
    // Set the endpoints for the filter graph. The filter_graph will
    // be linked to the graph described by filters_descr.
    

    
    // The buffer source output must be connected to the input pad of
    // the first filter described by filters_descr; since the first
    // filter input label is not specified, it is set to "in" by
    // default.
    
    outputs->name       = av_strdup("in");
    outputs->filter_ctx = *buffersrc_ctx;
    outputs->pad_idx    = 0;
    outputs->next       = NULL;

    
    // The buffer sink input must be connected to the output pad of
    // the last filter described by filters_descr; since the last
    // filter output label is not specified, it is set to "out" by
    // default.
    
    inputs->name       = av_strdup("out");
    inputs->filter_ctx = *buffersink_ctx;
    inputs->pad_idx    = 0;
    inputs->next       = NULL;

    if ((ret = avfilter_graph_parse_ptr(*filter_graph, filters_descr,
                                    &inputs, &outputs, NULL)) < 0)
        goto end;

    if ((ret = avfilter_graph_config(*filter_graph, NULL)) < 0)
        goto end;

end:
    avfilter_inout_free(&inputs);
    avfilter_inout_free(&outputs);

    return ret;
}

*/
import "C"
import (
	"unsafe"
	"runtime"
	"fmt"
	"image"
	//"image/color"

	"reflect"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/h264parser"
)

type VideoDecoder struct {
	ff *ffctx

	Extradata []byte
}

func (self *VideoDecoder) Setup() (err error) {
	ff := &self.ff.ff
	if len(self.Extradata) > 0 {
		ff.codecCtx.extradata = (*C.uint8_t)(unsafe.Pointer(&self.Extradata[0]))
		ff.codecCtx.extradata_size = C.int(len(self.Extradata))
	}
	if C.avcodec_open2(ff.codecCtx, ff.codec, nil) != 0 {
		err = fmt.Errorf("ffmpeg: decoder: avcodec_open2 failed")
		return
	}

	if C.init_filters(ff.codecCtx,&self.ff.g_filter_graph, &self.ff.g_buffersrc_ctx,&self.ff.g_buffersink_ctx) != 0 {
		err = fmt.Errorf("ffmpeg: filters init error")
		return
	}
	
	return
}

func fromCPtr(buf unsafe.Pointer, size int) (ret []uint8) {
	hdr := (*reflect.SliceHeader)((unsafe.Pointer(&ret)))
	hdr.Cap = size
	hdr.Len = size
	hdr.Data = uintptr(buf)
	return
}

type VideoFrame struct {
	Image image.YCbCr//image.RGBA
	Frame *C.AVFrame
	Pts int64
}

func (self *VideoFrame) Free() {
	self.Image = image.YCbCr{}
	C.av_frame_free(&self.Frame)
}

func freeVideoFrame(self *VideoFrame) {
	self.Free()
}

func (self *VideoDecoder) Decode(pkt []byte, dts int64, pts int64) (img *VideoFrame, err error) {
	ff := &self.ff.ff

	cgotimg := C.int(0)
	cpts := C.int64_t(0)

	frame := C.av_frame_alloc()
	frame_filtered := C.av_frame_alloc()

	cerr := C.wrap_avcodec_decode_video2(ff.codecCtx, frame, unsafe.Pointer(&pkt[0]), C.int(len(pkt)), C.int64_t(dts), C.int64_t(pts), &cgotimg, &cpts)
	

	
	if cerr < C.int(0) {
		err = fmt.Errorf("ffmpeg: avcodec_decode_video2 failed: %d", cerr)
		return
	}

	if cgotimg != C.int(0) {

		cerr = C.wrap_av_filter(self.ff.g_buffersrc_ctx,self.ff.g_buffersink_ctx, frame,frame_filtered);

		fmt.Println("w h ys cs",frame.format,frame.width,frame.height,frame.linesize[0],frame.linesize[1],frame.pict_type,frame.colorspace,frame.color_range,frame.buf,frame)

		cpts = frame.pts
		C.av_frame_free(&frame)

		if cerr >= 0 {
			fmt.Println("couocu",frame_filtered,frame_filtered.format)

			/*w := int(frame_filtered.width)
			h := int(frame_filtered.height)
			ys := int(frame_filtered.linesize[0])
			cs := int(frame_filtered.linesize[1])

			fmt.Println("w h ys cs",w,h,ys,cs,frame_filtered.pict_type,frame_filtered.colorspace,frame_filtered.color_range,frame_filtered.buf,frame_filtered)
			*/


			// For 4:4:4, CStride == YStride/1 && len(Cb) == len(Cr) == len(Y)/1.
			// For 4:2:2, CStride == YStride/2 && len(Cb) == len(Cr) == len(Y)/2.
			// For 4:2:0, CStride == YStride/2 && len(Cb) == len(Cr) == len(Y)/4.
			// For 4:4:0, CStride == YStride/1 && len(Cb) == len(Cr) == len(Y)/2.
			// For 4:1:1, CStride == YStride/4 && len(Cb) == len(Cr) == len(Y)/4.
			// For 4:1:0, CStride == YStride/4 && len(Cb) == len(Cr) == len(Y)/8.

			w := int(frame_filtered.linesize[0])
			h := int(frame_filtered.height)
			r := image.Rectangle{image.Point{0, 0}, image.Point{w, h}}
			// TODO: Use the sub sample ratio from the input image 'f.format'
			imgYCbCr := image.NewYCbCr(r, image.YCbCrSubsampleRatio420)
			// convert the frame data data to a Go byte array
			imgYCbCr.Y = C.GoBytes(unsafe.Pointer(frame_filtered.data[0]), C.int(w*h))

			wCb := int(frame_filtered.linesize[1])
			if unsafe.Pointer(frame_filtered.data[1]) != nil {
				imgYCbCr.Cb = C.GoBytes(unsafe.Pointer(frame_filtered.data[1]), C.int(wCb*h/2))
			}

			wCr := int(frame_filtered.linesize[2])
			if unsafe.Pointer(frame_filtered.data[2]) != nil {
				imgYCbCr.Cr = C.GoBytes(unsafe.Pointer(frame_filtered.data[2]), C.int(wCr*h/2))
			}
			
			


			/*
			w := int(frame_filtered.linesize[0])
			h := int(frame_filtered.height)
			r := image.Rectangle{image.Point{0, 0}, image.Point{w, h}}

			fmt.Println("w h ys cs",w,h,frame_filtered)

			// TODO: Use the sub sample ratio from the input image 'f.format'
			imgrgb := image.NewRGBA(r)
			// convert the frame data data to a Go byte array
			imgrgb.Pix = C.GoBytes(unsafe.Pointer(frame_filtered.data[0]), C.int(w*h))
			imgrgb.Stride = w
			fmt.Println("w", w, "h", h)
*/

			// Assume avFrame is the AVFrame with AVCOL_RANGE_JPEG color space encoding
			//img2 := image.NewRGBA(image.Rect(0, 0, int(frame_filtered.width), int(frame_filtered.height)))
			/*for y := 0; y < int(frame_filtered.height); y++ {
				for x := 0; x < int(frame_filtered.width); x++ {
					img2.SetNRGBA(x, y, color.NRGBA{
						R: uint8(frame_filtered.data[0])[y * int(frame_filtered.linesize[0])+ x]),
						G: uint8(frame_filtered.data[1])[y * int(frame_filtered.linesize[1]) + x]),
						B: uint8(frame_filtered.data[2])[y * int(frame_filtered.linesize[2]) + x]),
						A: 255,
					})
				}
			}*/
			img = &VideoFrame{Image:*imgYCbCr, Frame: frame_filtered}
			img.Pts = int64(cpts)//int64(frame.pts)
			if int(frame_filtered.interlaced_frame) == 1 {
				fmt.Println("interlaced",frame_filtered.pts,frame_filtered.pkt_dts,frame_filtered.display_picture_number)
			}
			runtime.SetFinalizer(img, freeVideoFrame)
		} else {
			fmt.Println("couocu bad",cerr)
		}
	}

	return
}

func NewVideoDecoder(stream av.CodecData) (dec *VideoDecoder, err error) {
	_dec := &VideoDecoder{}
	var id uint32

	switch stream.Type() {
	case av.H264:
		h264 := stream.(h264parser.CodecData)
		_dec.Extradata = h264.AVCDecoderConfRecordBytes()
		id = C.AV_CODEC_ID_H264

	default:
		err = fmt.Errorf("ffmpeg: NewVideoDecoder codec=%v unsupported", stream.Type())
		return
	}

	c := C.avcodec_find_decoder(id)
	if c == nil || C.avcodec_get_type(id) != C.AVMEDIA_TYPE_VIDEO {
		err = fmt.Errorf("ffmpeg: cannot find video decoder codecId=%d", id)
		return
	}

	if _dec.ff, err = newFFCtxByCodec(c); err != nil {
		return
	}
	if err =  _dec.Setup(); err != nil {
		return
	}

	dec = _dec
	return
}

