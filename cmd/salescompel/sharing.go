package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"salescompel/internal/platform"
)

// sharingCommand runs `salescompel sharing set-default|rule ...` (Plan 34):
// org-wide defaults and owner-based sharing rules, until a setup UI and
// metadata deploy exist.
func sharingCommand(ctx context.Context, s *platform.Store, args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: salescompel sharing set-default|rule add|rule list|rule remove")
	}
	fs := flag.NewFlagSet("sharing", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tenant := fs.String("tenant", "", "tenant")
	object := fs.String("object", "", "object API name")
	access := fs.String("access", "", "access")
	name := fs.String("name", "", "rule name")
	source := fs.String("source", "", "source group ID or developer name")
	target := fs.String("target", "", "target group ID or developer name")
	contact := fs.String("contact-access", "", "Account rules: contact access")
	id := fs.String("id", "", "rule ID")
	rest := args[2:]
	if args[1] == "rule" {
		if len(args) < 3 {
			return errors.New("usage: salescompel sharing rule add|list|remove")
		}
		rest = args[3:]
	}
	if e := fs.Parse(rest); e != nil || fs.NArg() != 0 {
		return errors.New("invalid sharing flags")
	}
	switch {
	case args[1] == "set-default":
		return s.SetDefaultAccess(ctx, *tenant, *object, *access)
	case args[1] == "rule" && args[2] == "add":
		ruleID, e := s.AddSharingRule(ctx, *tenant, platform.SharingRule{Object: *object, Name: *name, Source: *source, Target: *target, Access: *access, ContactAccess: *contact})
		if e != nil {
			return e
		}
		_, e = fmt.Fprintln(out, ruleID)
		return e
	case args[1] == "rule" && args[2] == "list":
		rules, e := s.SharingRules(ctx, *tenant)
		if e != nil {
			return e
		}
		for _, r := range rules {
			if _, e = fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.Object, r.Name, r.Source, r.Target, r.Access, r.ContactAccess); e != nil {
				return e
			}
		}
		return nil
	case args[1] == "rule" && args[2] == "remove":
		return s.RemoveSharingRule(ctx, *tenant, *id)
	}
	return errors.New("unknown sharing command")
}
