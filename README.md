# Hexcord mediaserver

### Hexcord mediaserver is a standalone server that allows you to initiate a peer connection and forward audio and video streams via WebRTC to an rtmp endpoint.

## __Instructions__
The mediaserver makes use of [ffmpeg](https://ffmpeg.org) to forward the streams so you would need that installed. You would find installation instructions for ffmpeg [here](https://ffmpeg.org/download.html). The [Pion webrtc](https://github.com/pion/webrtc) library was used for the WebRTC implementation.

Start the server by either running ```go run main.go``` or run ```go build``` to build the binaries instead.

The server starts up at [http://localhost:8090](http://localhost:8090) and exposes an endpoint ```http://localhost:8090/webrtc/offer```

Send a POST request with body
```
  {
    "ingestionAddress": "rtmp://RTMP_ADDRESS",
	"streamKey":        "234343sdsd-sdsfdfd-232dfdf",
	"offer":            "sdfarergehteafadfadfrrererdfdfdfd" // WebRTC offer from client
  }
```
Content-type for the request is ```application/json```

The endpoint responds with a JSON body containing a webrtc session description
```
  {
    "type": "answer",
    "sdp": "sdsawewsdf434-dsdf34-sfdsfdgs434.sfdfdbvererasdsds" // use this as the remote description of the peer connection initiated on the client
  }
```

Have fun ❤️