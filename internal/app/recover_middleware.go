package app

import (
	"log"
	"runtime/debug"

	tele "gopkg.in/telebot.v3"
)

func RecoverMiddleware() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("💥 PANIC [handler]: %v\n%s", r, string(debug.Stack()))
				}
			}()
			return next(c)
		}
	}
}
