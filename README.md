# JustProject

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest


字节序：binary.LittleEndian or BigEndian
	big：大端序，数据的低位字节放在内存的高位，符合阅读习惯
		例如int16(5): 0000 0000 0000 0101
	little：小端序，数据的低位字节放在内存的低位，与阅读习惯相反
		例如int16(5): 0000 0101 0000 0000 
		
	计算机电路是从低地址向高地址读取的，且CPU的运算是从数据低位开始的，
	这样在CPU读取内存数据时，读到了低地址的数据低位，就能立即送入ALU算术逻辑单元
	最大化利用了硬件带宽和并行能力