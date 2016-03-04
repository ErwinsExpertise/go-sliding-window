package swp

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}
