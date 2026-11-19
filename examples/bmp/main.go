// Generate an Akamai BMP sensor.
//
//	VEILROUTE_KEY=... go run ./examples/bmp com.example.app
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/veilroutesdk/veilroute-go"
)

func main() {
	app := "com.example.app"
	if len(os.Args) > 1 {
		app = os.Args[1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bmp, err := veilroute.NewClient().BMP.Sensor(ctx, veilroute.BMPSensorParams{App: app})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sensor generated in %d ms (SDK %s)\n", bmp.LatencyMs, bmp.SDKVersion)
	fmt.Printf("%s: %d chars\n", veilroute.HeaderName, len(bmp.Sensor))
}
