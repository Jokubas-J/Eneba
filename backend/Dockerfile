# How Render builds and runs the backend. Two stages: the first has the Go compiler,
# the second contains only the compiled program, so the image that runs is small and
# has no compiler, shell or package manager in it.

# --- Stage 1: compile ---------------------------------------------------------
FROM golang:1.27-alpine AS build
WORKDIR /src

# Copy only the dependency lists first. Docker caches each step, so dependencies are
# downloaded again only when go.mod/go.sum change, not on every code change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a statically linked binary that needs no C library, which is
# what lets the final image be almost empty. The migrations are embedded in the binary.
RUN CGO_ENABLED=0 go build -trimpath -o /server ./cmd/server

# --- Stage 2: run -------------------------------------------------------------
# "distroless static" holds just CA certificates (needed for TLS to Neon and Stripe),
# time zone data and a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server

# Render provides PORT; the server reads it from the environment.
ENTRYPOINT ["/server"]
