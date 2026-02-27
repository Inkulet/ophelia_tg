package app

var heavyLimiter = make(chan struct{}, 2)

func runHeavy(name string, fn func()) {
	safeGo(name, func() {
		heavyLimiter <- struct{}{}
		defer func() { <-heavyLimiter }()
		fn()
	})
}
