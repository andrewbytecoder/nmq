package runtimecfg

type ServerAddress struct {
	Name    string
	Address string
}

type Server struct {
	NcpHttpAddress  ServerAddress
	NcpHttpsAddress ServerAddress
	WebuiAddress    ServerAddress
}

func GetNcpHttpAddress() ServerAddress {
	return globalConfig.Server.NcpHttpAddress
}

func SetNcpHttpAddress(address ServerAddress) {
	globalConfig.Server.NcpHttpAddress = address
}

func GetNcpHttpsAddress() ServerAddress {
	return globalConfig.Server.NcpHttpsAddress
}

func SetNcpHttpsAddress(address ServerAddress) {
	globalConfig.Server.NcpHttpsAddress = address
}

func GetWebuiAddress() ServerAddress {
	return globalConfig.Server.WebuiAddress
}

func SetWebuiAddress(address ServerAddress) {
	globalConfig.Server.WebuiAddress = address
}
