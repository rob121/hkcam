package main

import (
	"flag"
	"fmt"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/log"
	"github.com/brutella/hkcam"
	"github.com/brutella/hkcam/ffmpeg"
	"github.com/rob121/vhelp"
	"image"
	"os"
	"runtime"
)

var Accessories []*accessory.Accessory
var dataDir *string


type Cameras map[string]Camera

type Camera struct{
	ID  uint64 `mapstructure:"id"`
	Device string `mapstructure:"input_device"`
	Loopback string
	Name string `mapstructure:"name"`
	FileName string `mapstructure:"filename"`
	H264Decoder string `mapstructure:"h264_decoder"`
	H264Encoder string `mapstructure:"h264_encoder"`
	MinVideoBitrate int `mapstructure:"min_video_bitrate"`
	MultiStream bool `mapstructure:"multistream"`
}

func main() {


	//base of filename hash
	var dataDir *string = flag.String("data_dir", "camera", "Path to data directory")
	var verbose *bool = flag.Bool("verbose", true, "Verbose logging")
	var pin *string = flag.String("pin", "11112222", "PIN for HomeKit pairing")
	var port *string = flag.String("port", "59876", "Port on which transport is reachable")

	vhelp.Load("cameras")
	conf,err := vhelp.Get("cameras")

	if(err!=nil){

		log.Info.Fatalf("%v",err)
	}


	flag.Parse()


	if *verbose {
		log.Debug.Enable()
		ffmpeg.EnableVerboseLogging()
	}


	var cameras Cameras

	conf.Unmarshal(&cameras)

	for _,c := range cameras {

		fmt.Printf("%#v",c)

		log.Info.Printf("Adding camera: %s",c.Name)

		camDefaults(&c)

		acc := addCam(c)
		Accessories = append(Accessories,acc)

	}



    for _,a := range Accessories {
		log.Info.Printf("%#v", a)
	}

	config := hc.Config{Pin: *pin, Port: *port, StoragePath: *dataDir}
	bridge := accessory.NewBridge(accessory.Info{Name: "Homekit IP Camera Bridge", ID: 1})

	transp,_ := hc.NewIPTransport(config,bridge.Accessory,Accessories...)



	hc.OnTermination(func() { <-transp.Stop()
		os.Exit(1)
	})


	transp.Start()


}

func camDefaults(c *Camera){

	if runtime.GOOS == "linux" {

		if(len(c.Device)<1){ c.Device = "v4l2"} //or rtsp, etc
		if(len(c.FileName)<1) { c.FileName = "/dev/video0"}
		if(len(c.Loopback)<1) { c.Loopback = "/dev/video1"}
		if(len(c.H264Decoder)<1) { c.H264Decoder = ""}
		if(len(c.H264Encoder)<1) { c.H264Encoder = "h264_omx" }

	} else if runtime.GOOS == "darwin" { // macOS


		if(len(c.Device)<1){ c.Device = "avfoundation"} //or rtsp, etc
		if(len(c.FileName)<1) { c.FileName = "default"}
		if(len(c.Loopback)<1) { c.Loopback = ""}
		if(len(c.H264Decoder)<1) { c.H264Decoder = "h264"}
		if(len(c.H264Encoder)<1) { c.H264Encoder = "libx264" }


	} else {
		log.Info.Fatalf("%s platform is not supported", runtime.GOOS)
	}


}

func addCam(camera Camera) *accessory.Accessory{


	switchInfo := accessory.Info{ID: camera.ID,Name: camera.Name, FirmwareRevision: "0.0.9", Manufacturer: "Rob Alfonso"}
	cam := accessory.NewCamera(switchInfo)

	cfg := ffmpeg.Config{
		InputDevice:      camera.Device,
		InputFilename:    camera.FileName,
		LoopbackFilename: camera.Loopback,
		H264Decoder:      camera.H264Decoder,
		H264Encoder:      camera.H264Encoder,
		MinVideoBitrate:  camera.MinVideoBitrate,
		MultiStream:      camera.MultiStream,
	}


	ffmpeg := hkcam.SetupFFMPEGStreaming(cam, cfg)

	// Add a custom camera control service to record snapshots
	cc := hkcam.NewCameraControl()
	cam.Control.AddCharacteristic(cc.Assets.Characteristic)
	cam.Control.AddCharacteristic(cc.GetAsset.Characteristic)
	cam.Control.AddCharacteristic(cc.DeleteAssets.Characteristic)
	cam.Control.AddCharacteristic(cc.TakeSnapshot.Characteristic)


	cc.CameraSnapshotReq = func(width, height uint) (*image.Image, error) {
		return ffmpeg.Snapshot(width, height)
	}

	return cam.Accessory


}
