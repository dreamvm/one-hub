package mockserver

import (
	"context"
	"errors"
	"net"
	"time"
)

// DependencyReachable tests only the three fixed services in the isolated fixture.
// It performs no authentication or application command and retains no connection.
func DependencyReachable(target string, dial func(context.Context, string, string) (net.Conn, error)) (bool, error) {
	switch target {
	case "database:3306", "database:5432", "cache:6379":
	default:
		return false, errors.New("only isolated fixture dependencies are allowed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := dial(ctx, "tcp", target)
	if err != nil {
		return false, nil
	}
	defer conn.Close()
	return true, nil
}
