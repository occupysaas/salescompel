package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"salescompel/internal/metadata"
	"salescompel/internal/platform"
	"strings"
	"time"
)

type repeatedStrings []string

func (r *repeatedStrings) String() string     { return strings.Join(*r, ",") }
func (r *repeatedStrings) Set(s string) error { *r = append(*r, s); return nil }

// stderr receives deprecation notices.
var stderr io.Writer = os.Stderr

func adminCommand(ctx context.Context, s *platform.Store, args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("admin subcommand required")
	}
	if args[0] == "org" {
		if len(args) != 2 || args[1] != "set" {
			return errors.New("org set accepts environment variables only")
		}
		return s.SetOrgDefaults(ctx, os.Getenv("SALESCOMPEL_TENANT"), metadata.OrgDefaults{
			TimeZone: os.Getenv("SALESCOMPEL_TIME_ZONE"), Locale: os.Getenv("SALESCOMPEL_LOCALE"), Language: os.Getenv("SALESCOMPEL_LANGUAGE")})
	}
	if args[0] == "user" {
		if len(args) != 2 {
			return errors.New("user commands accept environment variables only")
		}
		name := os.Getenv("SALESCOMPEL_USERNAME")
		switch args[1] {
		case "create":
			id, e := s.CreateUserWithProfile(ctx, os.Getenv("SALESCOMPEL_TENANT"), name, os.Getenv("SALESCOMPEL_PASSWORD"), cliProfile())
			if e != nil {
				return e
			}
			_, e = fmt.Fprintln(out, id)
			return e
		case "set-password":
			return s.SetPassword(ctx, name, os.Getenv("SALESCOMPEL_PASSWORD"))
		case "deactivate":
			return s.DeactivateUser(ctx, name)
		default:
			return errors.New("unknown user command")
		}
	}
	fs := flag.NewFlagSet("oauth-app", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if args[1] == "create" {
		var in platform.AppInput
		var redirects, users repeatedStrings
		fs.StringVar(&in.Tenant, "tenant", "", "tenant")
		fs.StringVar(&in.Name, "name", "", "name")
		fs.BoolVar(&in.Public, "public", false, "public app")
		fs.Var(&redirects, "redirect-uri", "redirect URI")
		fs.Var(&users, "jwt-user", "allowed username")
		grants := fs.String("grants", "authorization_code,refresh_token", "comma separated grants")
		scopes := fs.String("scopes", "api,web,refresh_token", "comma separated scopes")
		cert := fs.String("jwt-cert", "", "certificate file")
		fs.StringVar(&in.RunAs, "run-as", "", "run-as username")
		if e := fs.Parse(args[2:]); e != nil {
			return errors.New("invalid oauth-app flags")
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		in.RedirectURIs = redirects
		in.JWTUsers = users
		in.Grants = strings.FieldsFunc(*grants, func(c rune) bool { return c == ',' || c == ' ' })
		in.Scopes = strings.FieldsFunc(*scopes, func(c rune) bool { return c == ',' || c == ' ' })
		if *cert != "" {
			b, e := os.ReadFile(*cert)
			if e != nil {
				return errors.New("cannot read JWT certificate")
			}
			in.JWTCertPEM = string(b)
		}
		id, secret, e := s.CreateApp(ctx, in, time.Now())
		if e != nil {
			return e
		}
		v := map[string]string{"client_id": id}
		if secret != "" {
			v["client_secret"] = secret
		}
		return json.NewEncoder(out).Encode(v)
	}
	id := fs.String("client-id", "", "client ID")
	if e := fs.Parse(args[2:]); e != nil {
		return errors.New("invalid oauth-app flags")
	}
	if *id == "" || fs.NArg() != 0 {
		return errors.New("client ID required")
	}
	switch args[1] {
	case "rotate-secret":
		secret, e := s.RotateAppSecret(ctx, *id)
		if e != nil {
			return e
		}
		_, e = fmt.Fprintln(out, secret)
		return e
	case "disable":
		return s.DisableApp(ctx, *id)
	default:
		return errors.New("unknown oauth-app command")
	}
}

// cliProfile is SALESCOMPEL_PROFILE, or the profile for the deprecated
// SALESCOMPEL_ROLE (admin or restricted) when that is unset.
func cliProfile() string {
	if p := os.Getenv("SALESCOMPEL_PROFILE"); p != "" {
		return p
	}
	role := os.Getenv("SALESCOMPEL_ROLE")
	if role != "" {
		fmt.Fprintln(stderr, "SalesCompel: SALESCOMPEL_ROLE is deprecated; set SALESCOMPEL_PROFILE to a profile name instead")
	}
	return platform.RoleProfiles[role]
}
