package p2p;

import (
    "encoding/json"
    "fmt"
    "time"
    "context"
    "os"
    dhash "blockchain/hash"
    "crypto/rsa"
    "crypto/x509"
    "crypto"
    "encoding/base64"
    "blockchain/common"
    models "blockchain/models"
    "google.golang.org/grpc"
    "sync"
)

type utxo struct {
    spent map[string]bool
    hash  string
}

type P2P_Server struct {
    models.UnimplementedP2PServer
    PeerList *models.PeerList
    TransPool *models.TransPool
    NewTrans  chan *models.Trans
    clients []models.P2PClient
    localset []*utxo
}

func (s *P2P_Server) Init(){
    s.PeerList=new(models.PeerList)
    s.TransPool=new(models.TransPool)
    s.clients=make([]models.P2PClient,0)
    s.NewTrans=make(chan *models.Trans,1000)
    //从本地读取区块链，没有的话创建新的区块链
    models.CreateChain()
    data, err := os.ReadFile("./blockchain.json")
    if err!=nil{
        return
    }
    //解析区块链
    chain,err:=models.FormatChain(data)
    models.ReplaceChain(chain)
    if err!=nil{
        fmt.Println(err)
        return
    }
    //从区块链中解析交易池
    for _,block :=range chain.Chain{
        transpool,err:=models.FormatTrans(block.Hash)
        if err!=nil{
            fmt.Println(err)
            return
        }
        s.TransPool.TransPool=append(s.TransPool.TransPool,transpool.TransPool...)
    }
}

func (s *P2P_Server) SaveChainToLocal() error {
    println("save chain to local")
    data, err := json.Marshal(models.FetchChain())
    if err != nil {
        println("error in save chain to local")
        return err
    }

    //将json格式的区块链写入到本地
    err = os.WriteFile("./blockchain.json", data, 0644)
    if err != nil {
        return err
    }
    return nil
}

func (s *P2P_Server) RequestTailBlock(ctx context.Context, in *models.TailBlockRequest) (*models.Block, error) {
    return models.GetChainTail(), nil
}

//这里是UpdateBlockChain的接收方，目的是当server接受到了一个更长的链上的block，它去请求整条链，然后发送消息的server把整条链给它，这里是
//在处理接受到整条链的时候。
func (s *P2P_Server) UpdateBlockChain(ctx context.Context, in *models.BlockChain) (*models.UpdateBlockChainResponse, error) {
    dhash.StopHash()
    defer dhash.StartHash()
    err:=models.ReplaceChain(in)
    if err!=nil{
        return &models.UpdateBlockChainResponse{},common.Error(common.ErrInvalidChain)
    }
    err=s.SaveChainToLocal()
    if err!=nil{
        return &models.UpdateBlockChainResponse{},err
    }
    go s.Broadcast(models.GetChainTail())
    return &models.UpdateBlockChainResponse{},nil
}

func (s *P2P_Server) NewBlock(ctx context.Context, in *models.Block) (*models.NewBlockResponse, error) {
    time_start := time.Now()
    dhash.StopHash()
    defer dhash.StartHash()
    tailBlock := models.GetChainTail()
    if models.CompareBlock(in, tailBlock) {
        return &models.NewBlockResponse{ChainNeedUpdate: false}, nil
    }
    fmt.Println("new block received")
    if in.IsValid(tailBlock) {
        err := models.AppendChain(in)
        if err != nil {
            return &models.NewBlockResponse{ChainNeedUpdate: false}, err
        }
        if (len(in.Hash) > 0){
            //record filepath
            data, err := os.ReadFile("utxoset.json")
            json.Unmarshal(data, &s.localset)
            for k := 0; k < len(s.localset); k++ {
                fmt.Println("localset:",s.localset[k])
            }
            for i := 0; i < len(in.Hash); i++ {
                falsearray := make(map[string]bool)
                trans := models.FromJson([]byte(in.Hash[i]))
                fmt.Println("trans:",trans)
                for j := 0; j < len(trans.Tx_Outs); j++ {
                    falsearray[trans.Tx_Outs[j]] = false
                }
                newutxo := &utxo{spent: falsearray, hash: in.Hash[i]}
                s.localset = append(s.localset, newutxo)
                data, err := json.Marshal(s.localset)
                if err != nil {
                    fmt.Println("failed to write to utxoset.json: %v", err)
                    return &models.NewBlockResponse{ChainNeedUpdate: false}, err
                }
                err = os.WriteFile("./utxoset.json", data, 0644)
                if err != nil {
                    return &models.NewBlockResponse{ChainNeedUpdate: false}, err
                }

            }
            if err != nil {
                return &models.NewBlockResponse{ChainNeedUpdate: false}, err
            }
            time_end := time.Now()
            fmt.Println("time:",time_end.Sub(time_start))
            fmt.Println("time:",time_end.Sub(time_start).String())
            os.WriteFile("./time.txt", []byte(time_end.Sub(time_start).String()), 0644)
            writedata,err := json.Marshal(s.localset)
            os.WriteFile("./utxoset.json", writedata, 0644)
            fmt.Println("writedata:",writedata)
            fmt.Println("new block added, index:", in.Index)
        }
        go s.Broadcast(in)
        err = s.SaveChainToLocal()
        if err != nil {
            return &models.NewBlockResponse{ChainNeedUpdate: false}, err
        }
        return &models.NewBlockResponse{ChainNeedUpdate: false}, nil
    }
    
    if in.Index > tailBlock.Index+1 {
        return &models.NewBlockResponse{ChainNeedUpdate: true}, nil
    }
    return &models.NewBlockResponse{ChainNeedUpdate: false}, nil
}


func (s *P2P_Server) NewTransaction(ctx context.Context,in *models.Trans)(*models.NewTransactionResponse,error){
    fmt.Println("receiced trans in 183 1")
     valid:=make([]bool,in.NumInputs)
     for i:=0;i < int(in.NumInputs);i++{
            valid[i]=true
        }
        data, err := os.ReadFile("./utxoset.json")
        if err != nil {
            fmt.Println("error in read utxoset")
            fmt.Println(err)
            return &models.NewTransactionResponse{},err
        }
        err = json.Unmarshal(data, &s.localset)
        var wg sync.WaitGroup
        for i:=0;i < int(in.NumInputs);i++{
            wg.Add(1)
            for k := 0; k < len(s.localset); k++ {
                wg.Add(1)
                utxo := s.localset[k]
                fmt.Println("utxo:",utxo)
                trans := models.FromJson([]byte(utxo.hash))
                fmt.Println("trans:",trans)
                if trans.Signature == in.Tx_Ins[i]{
                    println("find utxo")
                    if utxo.spent[in.Pubkey]{
                        valid[i]=false
                        break
                    }
                    valid[i]=true
                }
            }
        

            for _,pvTrans := range s.TransPool.TransPool{
                if(pvTrans.Signature == in.Tx_Ins[i]){
                   beforesig := &models.Trans{
                        NumInputs:  pvTrans.NumInputs,
                        NumOutputs: pvTrans.NumOutputs,
                        Tx_Ins:     pvTrans.Tx_Ins,
                        Signature:  "",
                        PayOut:     pvTrans.PayOut,
                        Pubkey:     pvTrans.Pubkey,
                        Tx_Outs:    pvTrans.Tx_Outs,
                   }
                    UnhashedJson,err := beforesig.ToJson()
                    pb := pvTrans.Pubkey
                    pbKey, err := base64.StdEncoding.DecodeString(pb)
                    pubv, err := x509.ParsePKIXPublicKey(pbKey)
                    ciphertext, err := base64.StdEncoding.DecodeString(in.Tx_Ins[i])
                    if err != nil {
                        return &models.NewTransactionResponse{},err
                    }
                    pub := pubv.(*rsa.PublicKey)
                    err = rsa.VerifyPKCS1v15(pub, crypto.SHA256, UnhashedJson, []byte(ciphertext))
                    if err!=nil{
                        println("fail to unmarshal in pvTrans")
                        println(err)
                        return &models.NewTransactionResponse{},err
                    }
                    
                }   
                
            }
     }
     wg.Wait()
     for l:=0;l<len(valid);l++{
        if !valid[l]{
            fmt.Println("invalid trans attempt")
            return &models.NewTransactionResponse{},common.Error(common.ErrInvalidTrans)
        }
    }
    s.NewTrans<- in
    fmt.Println("trans received from")
    return &models.NewTransactionResponse{},nil
}


func (s *P2P_Server) ReadPeerListFromLocal() error {
    //读取以json格式存储的peerlist
    data, err := os.ReadFile("./peerlist.json")
    if err != nil {
        return err
    }
    //解析peerlist
    err = json.Unmarshal(data, s.PeerList)
    if err != nil {
        return err
    }
    return nil
}

func (s *P2P_Server) NewPeer(ctx context.Context,in *models.Peer)(*models.PeerList,error){
    //读取本地的peerlist
    err:=s.ReadPeerListFromLocal()
    if err!=nil{
        return nil,err
    }
    //检验重复
    for _,peer:=range s.PeerList.Peers{
        if peer==in{
            return s.PeerList,nil
        }
    }
    //将新的peer加入到peerlist中
    s.PeerList.Peers=append(s.PeerList.Peers,in)
    //将peerlist写入到本地
    err=s.WritePeerListToLocal()
    if err!=nil{
        return nil,err
    }
    return s.PeerList,nil 
}

func (s *P2P_Server) WritePeerListToLocal() error {
    //将peerlist转换成json格式
    data, err := json.Marshal(s.PeerList)
    if err != nil {
        return err
    }
    //将json格式的peerlist写入到本地
    err = os.WriteFile("./peerlist.json", data, 0644)
    if err != nil {
        return err
    }
    return nil
}

func (s *P2P_Server) SetupConnections()  error {
    time.Sleep(5*time.Second)
    //读取本地的peerlist
    err:=s.ReadPeerListFromLocal()
    if err!=nil{
        return err
    }
    //建立连接
    for _,peer:=range s.PeerList.Peers{
        conn,err:=grpc.Dial(fmt.Sprintf("%s:%d",peer.IP,peer.Port),grpc.WithInsecure())
        if err!=nil{
            continue
        }
        s.clients=append(s.clients,models.NewP2PClient(conn))
    }
    //启动监控
    //go s.monitorConnections()
    return nil
}



func (s *P2P_Server) Broadcast(block *models.Block) {
    fmt.Println(len(s.clients))
    for _, client := range s.clients {
        go func(client models.P2PClient) {
            res,err:=client.NewBlock(context.Background(),block)
            if err!=nil{
                fmt.Println(err)
            }else{
            if res.ChainNeedUpdate{
                _,err:=client.UpdateBlockChain(context.Background(),models.FetchChain())
                if err!=nil{
                    fmt.Println(err)
                }
            }}
        }(client)
    }
}







