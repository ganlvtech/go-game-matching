package matcher

type PlayerAlreadyExistsError PlayerId

func (e PlayerAlreadyExistsError) Error() string {
	return "player already exists. id = " + string(e)
}

type PlayerNotExistsError PlayerId

func (e PlayerNotExistsError) Error() string {
	return "player not exists. id = " + string(e)
}

type PlayerAlreadyMatchedError PlayerId

func (e PlayerAlreadyMatchedError) Error() string {
	return "player already matched. id = " + string(e)
}

type PlayerNotMatchedError PlayerId

func (e PlayerNotMatchedError) Error() string {
	return "player not matched. id = " + string(e)
}
