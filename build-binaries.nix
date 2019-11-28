with import <nixpkgs> {}; {
  devEnv = stdenv.mkDerivation {
    name = "dev";
    buildInputs = [ stdenv go glibc.static ];
    CFLAGS="-I${pkgs.glibc.dev}/include";
    LDFLAGS="-L${pkgs.glibc}/lib";
    shellHook = ''
      env GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -linkmode external -extldflags -static" -o jb-linux-amd64 jb.go
      env CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ GOOS=windows GOARCH=amd64 go build -o jb-windows-amd64.exe jb.go
      env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -extldflags -static" -o jb-darwin-amd64 jb.go
    '';
  };
}
