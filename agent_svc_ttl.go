package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	consul "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Setup logging.
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Setup the Consul client.
	client, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		log.Fatalf("There was an error configuring Consul client: %q\n", err.Error())
	}

	// Deregister all registered services.
	svcs, _ := client.Agent().Services()
	for k := range svcs {
		client.Agent().ServiceDeregister(k)
	}
	log.Infoln("Deregistered all agent services from Consul.")

	// Register services with same TTL
	_ = client.Agent().ServiceRegister(&consul.AgentServiceRegistration{
		ID:   "svc-a-id",
		Name: "svc-a",
		Checks: []*consul.AgentServiceCheck{
			{
				CheckID:                        "svc-a-ttl",
				DeregisterCriticalServiceAfter: "1m",
				TTL:                            "20s",
			},
		},
	})
	_ = client.Agent().UpdateTTL("svc-a-ttl", "", consul.HealthPassing)
	log.Infoln("Registered svc-a")

	_ = client.Agent().ServiceRegister(&consul.AgentServiceRegistration{
		ID:   "svc-b-id",
		Name: "svc-b",
		Checks: []*consul.AgentServiceCheck{
			{
				CheckID:                        "svc-b-ttl",
				DeregisterCriticalServiceAfter: "1m",
				TTL:                            "20s",
			},
		},
	})
	_ = client.Agent().UpdateTTL("svc-b-ttl", "", consul.HealthPassing)
	log.Infoln("Registered svc-b")

	// Setup a ticket to wake up every n seconds.
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	log.Infoln()
	log.Infoln("******************************")
	log.Infoln("Each 10 seconds, we'll be refreshing [svc-a-tll] but NOT [svc-b-tll].")
	log.Infoln("After 20 seconds, [svc-b-tll] should be marked as critical.")
	log.Infoln("After NO MORE THAN 90 seconds, [svc-b-tll] and [svc-b] SHOULD be deleted from Consul.")
	log.Infoln("******************************")
	log.Infoln()

	// On every tick, reset one registered service TTL and list registered services.
	// The service which TTL is not reset should be gone after a while.
	go func() {
		for range ticker.C {
			// Reset svc-a-ttl TTL clock.
			_ = client.Agent().UpdateTTL("svc-a-ttl", "", consul.HealthPassing)
			log.Infoln("Reset [svc-a-ttl] TTL clock.")

			// List registered services.
			log.Infoln("Listing checks for registered services...")
			checks, _ := client.Agent().Checks()
			for _, v := range checks {
				log.Infof("--> Service: [%q], Check: [%q], Status: [%q]\n", v.ServiceID, v.CheckID, v.Status)
			}
			log.Println()
		}
	}()

	stopCh := make(chan struct{})
	termCh := make(chan os.Signal, 2)
	// Notify termCh of SIGINT and SIGTERM.
	signal.Notify(termCh, syscall.SIGINT, syscall.SIGTERM)
	// Wait for a signal to be received.
	go func() {
		<-termCh
		// The first signal was received, so we close the channel.
		close(stopCh)
		<-termCh
		// The second signal was received, so we exit immediately.
		os.Exit(1)
	}()

	<-stopCh
	log.Infoln("Done.")
}
