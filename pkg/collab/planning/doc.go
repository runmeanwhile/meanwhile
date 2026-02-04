// Package planning provides primitives for creating structured implementation plans.
//
// Planning is a collaboration-kit component that invokes a planning agent to produce
// a structured Plan document. The plan can be stored in session metadata, persisted
// to memory, or returned directly to the caller.
//
// The planning primitive does not execute plans - it creates them. Execution is handled
// by the main agent or protocol using the plan as context.
//
// Basic usage:
//
//	planner := planning.New(planningAgent)
//	plan, err := planner.CreatePlan(ctx, sess, message.User("Build REST API"))
//
// With storage:
//
//	planner := planning.New(
//	    planningAgent,
//	    planning.WithStorage(planning.StorageSession, "plan"),
//	)
//
// With approval:
//
//	planner := planning.New(
//	    planningAgent,
//	    planning.WithApproval(
//	        planning.WithTimeout(30*time.Second),
//	    ),
//	)
package planning
