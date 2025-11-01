package main

import (
	"encoding/hex"
	"fmt"
	"go-mesh/pb/meshtastic"
	"google.golang.org/protobuf/proto"
)

func main() {
	// Your hex data from !00000000
	hexData := "124b0d34b4515b15ffffffff22210803121b0d0060d01f15006040fc180025b258d56828017800" +
		"800100b80112480135c81818bb3db758d56845000010404801609affffffffffffffff01780396" +
		"58d5684803580a017805480025000020402d1a58d5683214085b15a24582401d9a998d4125a3b7" +
		"e53e28d01c48010851153bdf7f401d0000803f25a7e7c23f28f39c09"
	
	fmt.Printf("Analyzing hex data: %s\n", hexData)
	fmt.Printf("Length: %d characters (%d bytes)\n", len(hexData), len(hexData)/2)
	
	// Convert hex to bytes
	data, err := hex.DecodeString(hexData)
	if err != nil {
		fmt.Printf("Error decoding hex: %v\n", err)
		return
	}
	
	fmt.Printf("\nRaw bytes: %d bytes\n", len(data))
	fmt.Printf("First 32 bytes: %X\n", data[:min(len(data), 32)])
	
	// Try to parse as FromRadio protobuf message
	fmt.Println("\n=== Attempting to parse as FromRadio ===")
	
	fromRadio := &pb.FromRadio{}
	if err := proto.Unmarshal(data, fromRadio); err != nil {
		fmt.Printf("Failed to parse as FromRadio: %v\n", err)
		
		// Maybe it's a MeshPacket directly?
		fmt.Println("\n=== Attempting to parse as MeshPacket ===")
		meshPacket := &pb.MeshPacket{}
		if err := proto.Unmarshal(data, meshPacket); err != nil {
			fmt.Printf("Failed to parse as MeshPacket: %v\n", err)
		} else {
			fmt.Printf("Successfully parsed as MeshPacket!\n")
			fmt.Printf("From: %08x\n", meshPacket.GetFrom())
			fmt.Printf("To: %08x\n", meshPacket.GetTo())
			fmt.Printf("ID: %d\n", meshPacket.GetId())
			
			// Check payload
			payload := meshPacket.GetPayloadVariant()
			switch p := payload.(type) {
			case *pb.MeshPacket_Decoded:
				decoded := p.Decoded
				fmt.Printf("Decoded payload: portnum=%d\n", decoded.GetPortnum())
				fmt.Printf("Payload size: %d bytes\n", len(decoded.GetPayload()))
				
				// Try to decode based on portnum
				if decoded.GetPortnum() == 4 { // NODEINFO_APP
					fmt.Println("This is a NODEINFO packet!")
					user := &pb.User{}
					if err := proto.Unmarshal(decoded.GetPayload(), user); err == nil {
						fmt.Printf("User ID: %s\n", user.GetId())
						fmt.Printf("Long Name: %s\n", user.GetLongName())
						fmt.Printf("Short Name: %s\n", user.GetShortName())
						fmt.Printf("Hardware Model: %s\n", user.GetHwModel())
					}
				}
			case *pb.MeshPacket_Encrypted:
				fmt.Printf("Encrypted payload: %d bytes\n", len(p.Encrypted))
			}
		}
	} else {
		fmt.Printf("Successfully parsed as FromRadio!\n")
		fmt.Printf("ID: %d\n", fromRadio.GetId())
		
		// Check what kind of payload it has
		payload := fromRadio.GetPayloadVariant()
		switch p := payload.(type) {
		case *pb.FromRadio_Packet:
			fmt.Println("Contains MeshPacket")
			packet := p.Packet
			fmt.Printf("  From: %08x\n", packet.GetFrom())
			fmt.Printf("  To: %08x\n", packet.GetTo())
			fmt.Printf("  ID: %d\n", packet.GetId())
		case *pb.FromRadio_MyInfo:
			fmt.Println("Contains MyNodeInfo")
			myInfo := p.MyInfo
			fmt.Printf("  My Node Num: %08x\n", myInfo.GetMyNodeNum())
			fmt.Printf("  Reboot Count: %d\n", myInfo.GetRebootCount())
		case *pb.FromRadio_NodeInfo:
			fmt.Println("Contains NodeInfo")
			nodeInfo := p.NodeInfo
			fmt.Printf("  Node Num: %08x\n", nodeInfo.GetNum())
			if user := nodeInfo.GetUser(); user != nil {
				fmt.Printf("  User ID: %s\n", user.GetId())
				fmt.Printf("  Long Name: %s\n", user.GetLongName())
				fmt.Printf("  Short Name: %s\n", user.GetShortName())
			}
		case *pb.FromRadio_Config:
			fmt.Println("Contains Config")
		case *pb.FromRadio_LogRecord:
			fmt.Println("Contains LogRecord")
		default:
			fmt.Printf("Unknown payload type: %T\n", payload)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
