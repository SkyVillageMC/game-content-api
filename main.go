package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/SkyVillageMc/game-content-api/db"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

var API_KEY string

var SELF_URL string

func main() {
	db.Connect()
	defer db.Disconnect()

	if err := os.MkdirAll("./data/maps", os.ModeDir); err != nil {
		panic(err.Error())
	}

	if err := os.MkdirAll("./data/extensions", os.ModeDir); err != nil {
		panic(err.Error())
	}

	key, has := os.LookupEnv("API_KEY")
	if !has {
		panic("\"API_KEY\" environmental variable is missing")
	}
	API_KEY = key

	key, has = os.LookupEnv("SELF_URL")
	if !has {
		panic("\"SELF_URL\" environmental variable is missing")
	}
	SELF_URL = key

	app := fiber.New()

	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(func(c *fiber.Ctx) error {
		if c.Query("key") != API_KEY {
			return c.Status(403).SendString("incorrect api key!")
		}
		return c.Next()
	})

	initRoutes(app.Group("/api"))

	app.Listen(":8080")
}

type ContentUploadRequest struct {
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Description string `json:"description"`
	Data        string `json:"data"`
}

type ServerCreateRequest struct {
	Terminal         bool     `json:"terminal"`
	Compression      int      `json:"compressionThreshold"`
	Brand            string   `json:"brand"`
	Map              string   `json:"map"`
	Extensions       []string `json:"extensions"`
	Presistent       bool     `json:"presistent"`
	ForwardingSecret string   `json:"forwarding_secret"`
}

func initRoutes(r fiber.Router) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	//TODO remove this or something idk
	/* networks, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		panic(err)
	}
	hasCustomNet := false
	for _, v := range networks {
		if v.Labels["sv.type"] == "minecraft" {
			hasCustomNet = true
		}
	}
	if !hasCustomNet {
		log.Println("Custom network not found, creating one")
		_, err = cli.NetworkCreate(ctx, "mc-servers", types.NetworkCreate{
			Labels: map[string]string{
				"sv.type": "minecraft",
			},
			Driver: "bridge",
		})
		if err != nil {
			panic(err)
		}
	} */

	r.Get("/server/:id", func(c *fiber.Ctx) error {
		res, err := db.DB.Server.FindFirst(
			db.Server.ID.Equals(c.Params("id")),
		).With(
			db.Server.Map.Fetch(),
		).Exec(ctx)

		if err != nil {
			return c.Status(500).SendString("something is wrong or idk")
		}

		return c.JSON(fiber.Map{
			"id":                   res.ID,
			"terminal":             res.Terminal,
			"compressionThreshold": res.CompressionThreshold,
			"extensions":           make([]string, 0),
			"map":                  fmt.Sprintf("%s:%s", res.Map().Name, res.Map().Tag),
			"host":                 "0.0.0.0",
			"port":                 25565,
			"brand":                res.Brand,
		})
	})

	r.Get("/maps", func(c *fiber.Ctx) error {
		res, err := db.DB.Map.FindMany().Exec(ctx)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}
		return c.JSON(res)
	})

	r.Get("/maps/:name", func(c *fiber.Ctx) error {
		res, err := db.DB.Map.FindMany(
			db.Map.Name.Equals(c.Params("name")),
		).Exec(ctx)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}
		return c.JSON(res)
	})

	r.Post("/maps", func(c *fiber.Ctx) error {
		data := new(ContentUploadRequest)
		if err := c.BodyParser(&data); err != nil {
			log.Println(err.Error())
			return c.Status(400).SendString("some error or idk")
		}

		res, err := db.DB.Map.CreateOne(
			db.Map.Name.Set(data.Name),
			db.Map.Tag.Set(data.Tag),
			db.Map.Description.Set(data.Description),
		).Exec(ctx)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}

		raw, err := base64.StdEncoding.DecodeString(data.Data)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}
		err = os.WriteFile(fmt.Sprintf("./data/maps/%s.svw", res.ID), raw, os.ModePerm)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}

		return c.JSON(res)
	})

	r.Get("/servers", func(c *fiber.Ctx) error {
		res, err := db.DB.Server.FindMany().With(db.Server.Map.Fetch()).Exec(ctx)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}
		return c.JSON(res)
	})

	r.Post("/servers", func(c *fiber.Ctx) error {
		data := new(ServerCreateRequest)
		if err := c.BodyParser(&data); err != nil {
			return c.Status(400).SendString(`some error or idk`)
		}

		mapData := strings.Split(data.Map, `:`)

		mapRes, err := db.DB.Map.FindFirst(
			db.Map.Name.Equals(mapData[0]),
			db.Map.Tag.Equals(mapData[1]),
		).Exec(ctx)
		if err != nil {
			log.Println(err.Error())
			return c.Status(500).SendString("some error or idk")
		}

		res, err := db.DB.Server.CreateOne(
			db.Server.Map.Link(
				db.Map.ID.Equals(mapRes.ID),
			),
			db.Server.Terminal.Set(data.Terminal),
			db.Server.Brand.Set(data.Brand),
			db.Server.CompressionThreshold.Set(data.Compression),
		).Exec(ctx)

		if err != nil {
			log.Println(err.Error())
			return c.Status(500).SendString("some error or idk")
		}

		for _, v := range data.Extensions {
			name := strings.Split(v, ":")[0]
			tag := strings.Split(v, ":")[1]

			_, err = db.DB.ServerExtension.CreateOne(
				db.ServerExtension.Server.Link(
					db.Server.ID.Equals(res.ID),
				),
				db.ServerExtension.Extension.Link(
					db.Extension.And(
						db.Extension.Name.Equals(name),
						db.Extension.Tag.Equals(tag),
					),
				),
			).Exec(ctx)
			if err != nil {
				log.Println(err)
				//Delete everything if there's an error
				_, _ = db.DB.ServerExtension.FindMany(
					db.ServerExtension.Server.Where(
						db.Server.ID.Equals(res.ID),
					),
				).Delete().Exec(ctx)

				_, _ = db.DB.Server.FindMany(
					db.Server.ID.Equals(res.ID),
				).Delete().Exec(ctx)

				return c.Status(500).SendString("some error or idk")
			}
		}

		env := []string{
			fmt.Sprintf("SERVER_ID=%s", res.ID),
			fmt.Sprintf("SERVER_KEY=%s", API_KEY),
			fmt.Sprintf("SERVER_URL=%s", SELF_URL),
			fmt.Sprintf("PRESISTENT=%t", data.Presistent),
		}

		if data.ForwardingSecret != "" {
			env = append(env, fmt.Sprintf("FORWARDING_SECRET=%s", data.ForwardingSecret))
		}

		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:     "skystom",
			Tty:       true,
			OpenStdin: true,
			Hostname:  res.ID,
			Env:       env,
		}, nil, &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"mc-servers": {},
			},
		}, nil, fmt.Sprintf("server-%s", res.ID))
		if err != nil {
			panic(err)
		}

		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			panic(err)
		}

		return c.JSON(res)
	})

	r.Delete("/servers/:id", func(c *fiber.Ctx) error {
		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
			All: true,
		})
		if err != nil {
			return c.Status(500).SendString(`some error or idk`)
		}
		var container *types.Container = nil
		toFind := fmt.Sprintf("server-%s", c.Params("id"))
		for _, v := range containers {
			if v.Names[0] == toFind {
				container = &v
				break
			}
		}
		if container == nil {
			return c.Status(500).SendString(`some error or idk`)
		}

		timeout := time.Second * 5

		if container.State == "running" {
			err = cli.ContainerStop(ctx, container.ID, &timeout)
			if err != nil {
				return c.Status(500).SendString(`some error or idk`)
			}
		}

		err = cli.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{})
		if err != nil {
			return c.Status(500).SendString(`some error or idk`)
		}

		_, err = db.DB.Server.FindMany(
			db.Server.ID.Equals(c.Params("id")),
		).Delete().Exec(ctx)

		if err != nil {
			return c.Status(500).SendString(`some error or idk`)
		}

		return c.SendString("success")
	})

	r.Get("/extensions", func(c *fiber.Ctx) error {
		res, err := db.DB.Extension.FindMany().Exec(ctx)
		if err != nil {
			return c.Status(500).SendString(`some error or idk`)
		}
		return c.JSON(res)
	})

	r.Post("/extensions", func(c *fiber.Ctx) error {
		data := new(ContentUploadRequest)
		if err := c.BodyParser(&data); err != nil {
			return c.Status(400).SendString(`some error or idk`)
		}

		res, err := db.DB.Extension.CreateOne(
			db.Extension.Name.Set(data.Name),
			db.Extension.Tag.Set(data.Tag),
			db.Extension.Description.Set(data.Description),
		).Exec(ctx)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}

		raw, err := base64.StdEncoding.DecodeString(data.Data)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}
		err = os.WriteFile(fmt.Sprintf("./data/extensions/%s.jar", res.ID), raw, os.ModePerm)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}

		return c.JSON(res)
	})

	r.Get("/extension/:name/:tag", func(c *fiber.Ctx) error {
		res, err := db.DB.Extension.FindFirst(
			db.Extension.Name.Equals(c.Params("name")),
			db.Extension.Tag.Equals(c.Params("tag")),
		).Exec(ctx)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}

		return c.SendFile(fmt.Sprintf("./data/extensions/%s.jar", res.ID))
	})

	r.Get("/map/:name/:tag", func(c *fiber.Ctx) error {
		res, err := db.DB.Map.FindFirst(
			db.Map.Name.Equals(c.Params("name")),
			db.Map.Tag.Equals(c.Params("tag")),
		).Exec(ctx)
		if err != nil {
			return c.Status(500).SendString("some error or idk")
		}

		return c.SendFile(fmt.Sprintf("./data/maps/%s.svw", res.ID))
	})
}
