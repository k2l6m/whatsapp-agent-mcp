package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {

	logger := waLog.Stdout("Client", "INFO", true)

	dbLog := waLog.Stdout("Database", "INFO", true)
	
	err := os.MkdirAll("store", 0755)
	if err != nil {
		panic(err)
	}

	container, err := sqlstore.New(
		context.Background(),
		"sqlite3",
		"file:store/whatsapp.db?_foreign_keys=on",
		dbLog,
	)

	if err != nil {
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice(
		context.Background(),
	)

	if err != nil {
		deviceStore = container.NewDevice()
	}

	client := whatsmeow.NewClient(deviceStore, logger)

	if client.Store.ID == nil {

		qrChan, _ := client.GetQRChannel(context.Background())

		err = client.Connect()

		if err != nil {
			panic(err)
		}

		for evt := range qrChan {

			if evt.Event == "code" {

				fmt.Println("Scan QR:")

				qrterminal.GenerateHalfBlock(
					evt.Code,
					qrterminal.L,
					os.Stdout,
				)

			} else if evt.Event == "success" {

				fmt.Println("Connected!")
				break
			}
		}

	} else {

		err = client.Connect()

		if err != nil {
			panic(err)
		}

		fmt.Println("Already authenticated")
	}

	select {}
}
