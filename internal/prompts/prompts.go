package prompts

import (
	_ "embed"
)

// Director prompts
//
//go:embed director_system.txt
var DirectorSystem string

//go:embed director_task.txt
var DirectorTaskTemplate string

// Protagonist prompts
//
//go:embed protagonist_system.txt
var ProtagonistSystem string

//go:embed protagonist_task.txt
var ProtagonistTaskTemplate string

// Chapter prompts
//
//go:embed chapter_system.txt
var ChapterSystem string

//go:embed chapter_task.txt
var ChapterTaskTemplate string

// Premise prompts
//
//go:embed premise_system.txt
var PremiseSystem string

//go:embed premise_task.txt
var PremiseTaskTemplate string

// Mutual verification prompts.
// The Protagonist verifies the Director's event; the Director verifies the
// Protagonist's reaction.

//go:embed verify_event_system.txt
var VerifyEventSystem string

//go:embed verify_event_task.txt
var VerifyEventTaskTemplate string

//go:embed verify_reaction_system.txt
var VerifyReactionSystem string

//go:embed verify_reaction_task.txt
var VerifyReactionTaskTemplate string

// Revision prompts (system prompts are reused from the respective agents).

//go:embed revise_event_task.txt
var ReviseEventTaskTemplate string

//go:embed revise_reaction_task.txt
var ReviseReactionTaskTemplate string
