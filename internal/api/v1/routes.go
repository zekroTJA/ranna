package v1

import (
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/ranna-go/ranna/internal/config"
	"github.com/ranna-go/ranna/internal/sandbox"
	"github.com/ranna-go/ranna/internal/spec"
	"github.com/ranna-go/ranna/internal/static"
	"github.com/ranna-go/ranna/internal/util"
	"github.com/ranna-go/ranna/pkg/models"
	"github.com/sarulabs/di/v2"
)

var (
	errOutputLenExceeded = fiber.NewError(fiber.StatusBadRequest, "output len exceeded")
	errEmptyCode         = fiber.NewError(fiber.StatusBadRequest, "code is empty")
)

type Router struct {
	spec    spec.Provider
	cfg     config.Provider
	manager sandbox.Manager
}

func (r *Router) Setup(route fiber.Router, ctn di.Container) {
	r.cfg = ctn.Get(static.DiConfigProvider).(config.Provider)
	r.spec = ctn.Get(static.DiSpecProvider).(spec.Provider)
	r.manager = ctn.Get(static.DiSandboxManager).(sandbox.Manager)

	route.Use(r.optionsBypass)

	route.Get("/spec", r.getSpec)
	route.Post("/exec", r.postExec)
	route.Get("/info", r.getInfo)
}

func (r *Router) optionsBypass(ctx *fiber.Ctx) error {
	if ctx.Method() == "OPTIONS" {
		return ctx.SendStatus(fiber.StatusOK)
	}
	return ctx.Next()
}

func (r *Router) getInfo(ctx *fiber.Ctx) (err error) {
	sandboxInfo, err := r.manager.GetProvider().Info()
	if err != nil {
		return
	}
	info := &models.SystemInfo{
		SandboxInfo: sandboxInfo,
		Version:     static.Version,
		BuildDate:   static.BuildDate,
		GoVersion:   runtime.Version(),
	}
	return ctx.JSON(info)
}

func (r *Router) getSpec(ctx *fiber.Ctx) (err error) {
	return ctx.JSON(r.spec.Spec().GetSnapshot())
}

func (r *Router) postExec(ctx *fiber.Ctx) (err error) {
	req := new(models.ExecutionRequest)
	if err = ctx.BodyParser(req); err != nil {
		return
	}

	if req.Code == "" {
		return errEmptyCode
	}

	res, err := r.manager.RunInSandbox(req)
	if err != nil {
		if sandbox.IsSystemError(err) {
			return err
		}
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err = r.checkOutputLen(res.StdOut, res.StdErr); err != nil {
		return
	}

	return ctx.JSON(res)
}

func (r *Router) checkOutputLen(stdout, stderr string) (err error) {
	max, err := util.ParseMemoryStr(r.cfg.Config().API.MaxOutputLen)
	if err != nil {
		return
	}
	if int64(len(stdout))+int64(len(stderr)) > max {
		err = errOutputLenExceeded
	}
	return
}
