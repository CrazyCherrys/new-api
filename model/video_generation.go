package model

import "github.com/QuantumNous/new-api/constant"

var videoTaskActions = []string{
	constant.TaskActionGenerate,
	constant.TaskActionTextGenerate,
	constant.TaskActionFirstTailGenerate,
	constant.TaskActionReferenceGenerate,
	constant.TaskActionRemix,
}

func VideoTaskActions() []string {
	return append([]string(nil), videoTaskActions...)
}
