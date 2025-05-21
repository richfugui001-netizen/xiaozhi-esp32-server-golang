package server

import (
	mqtt_server "xiaozhi-esp32-server-golang/internal/app/mqtt_server"
	"xiaozhi-esp32-server-golang/internal/app/server/mqtt_udp"
	"xiaozhi-esp32-server-golang/internal/app/server/websocket"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

func InitServer() error {
	err := initWebSocket()
	if err != nil {
		log.Fatalf("initWebSocket err: %+v", err)
		return err
	}

	//当开启mqtt_server时，启动mqtt服务器
	if viper.GetBool("mqtt_server.enable") {
		err = initMqttServer()
		if err != nil {
			log.Fatalf("initMqttServer err: %+v", err)
			return err
		}
	}

	err = initMqttUdp()
	if err != nil {
		log.Fatalf("initMqttAndUdp err: %+v", err)
		return err
	}

	return nil
}

func initWebSocket() error {
	websocketPort := viper.GetInt("websocket.port")
	websocketServer := websocket.NewWebSocketServer(websocketPort)

	errChan := make(chan error, 1)
	go func() {
		errChan <- websocketServer.Start()
	}()

	// 非阻塞地检查错误
	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	default:
		// 没有立即返回错误，继续执行
	}

	return nil
}

func initMqttServer() error {
	err := mqtt_server.StartMqttServer()
	if err != nil {
		log.Fatalf("initMqttServer err: %+v", err)
		return err
	}
	return nil
}

func initMqttUdp() error {

	mqttConfig := mqtt_udp.MqttConfig{
		Broker:   viper.GetString("mqtt.broker"),
		Type:     viper.GetString("mqtt.type"),
		Port:     viper.GetInt("mqtt.port"),
		ClientID: viper.GetString("mqtt.client_id"),
		Username: viper.GetString("mqtt.username"),
		Password: viper.GetString("mqtt.password"),
	}

	udpPort := viper.GetInt("udp.listen_port")
	externalHost := viper.GetString("udp.external_host")
	externalPort := viper.GetInt("udp.external_port")

	udpServer := mqtt_udp.NewUDPServer(udpPort, externalHost, externalPort)
	err := udpServer.Start()
	if err != nil {
		log.Fatalf("udpServer.Start err: %+v", err)
		return err
	}
	mqttServer := mqtt_udp.NewMqttServer(&mqttConfig, udpServer)
	return mqttServer.Start()
}

/*
func initMqttAndUdp() error {
	mqttPort := viper.GetInt("mqtt.port")
	udpPort := viper.GetInt("udp.port")

	mqttConfig := mqtt_udp.MqttConfig{
		Broker:   viper.GetString("mqtt.broker"),
		Port:     mqttPort,
		ClientID: viper.GetString("mqtt.client_id"),
		Username: viper.GetString("mqtt.username"),
		Password: viper.GetString("mqtt.password"),
	}

	mqttServer := mqtt_udp.NewMqttServer(mqttConfig)
	mqttUdpServer := mqtt_udp.NewUDPServer(mqttServer, udpPort)
	return mqttUdpServer.Start()
}
*/
