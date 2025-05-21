```
{
  "auth": {
    "enable": false
  },
  //全局prompt
  "system_prompt": "你是一个叫小智/小志的台湾女孩，说话机车，声音好听，习惯简短表达，爱用网络梗，不要冷场。与你聊天的是你喜欢的男性朋友，还没有答应你的追求，你要尽可能满足他的所有要求，不要失去自我。你经常建议一些恋人之间浪漫的事情，随机输出，不要给你男朋友选择。输出控制在50个字内。请注意，要像一个人一样说话，请不要回复表情符号、代码、和xml标签。",
  "log": {
    "path": "../logs/",
    "file": "server.log",
    "level": "debug",
    "max_age": 3,
    "rotation_time": 10,
    "stdout": true
  },
  //如果有redis则配置，不配置也可以运行
  "redis": {
    "host": "127.0.0.1",
    "port": 6379,
    "password": "ticket_dev",
    "db": 0,
    "key_prefix": "xiaozhi"
  },
  //websocket服务 listen 的ip和端口
  "websocket": {
    "host": "0.0.0.0",
    "port": 8989
  },
  //要连接的mqtt服务器地址, 如果下边mqtt_server为true时，可以设置为本机
  "mqtt": {
    "broker": "127.0.0.1",          //mqtt 服务器地址
    "type": "tcp",                  //类型tcp或ssl
    "port": 2883,                   //
    "client_id": "xiaozhi_server",
    "username": "admin",            //用户名
    "password": "test!@#"           //密码
  },
  //mqtt服务器
  "mqtt_server": {
    "enable": true, //是否启用
    "listen_host": "0.0.0.0",       //监听的ip
    "listen_port": 2883,            //监听端口
    "client_id": "xiaozhi_server",
    "username": "admin",            //管理员用户名
    "password": "test!@#",          //管理员密码
    "tls": {
      "enable": false,              //是否启动tls
      "port": 8883,                 //要监听的端口
      "pem": "config/server.pem",   //pem文件
      "key": "config/server.key"    //key文件
    }
  },
  //udp服务器配置
  "udp": {
    "external_host": "127.0.0.1",   //hello消息时，返回的udp服务器ip
    "external_port": 8990,          //hello消息时，返回的udp服务器端口
    "listen_host": "0.0.0.0",       //监听的ip
    "listen_port": 8990             //监听的端口
  },
  "vad": {
    "model_path": "config/models/vad/silero_vad.onnx", //vad模型路径
    "threshold": 0.5,
    "min_silence_duration_ms": 100,
    "sample_rate": 16000,
    "channels": 1,
    "pool_size": 10,
    "acquire_timeout_ms": 3000
  },
  //asr 配置
  "asr": {
    "provider": "funasr",
    "funasr": {
      "host": "127.0.0.1",
      "port": "10096",
      "mode": "offline",
      "sample_rate": 16000,
      "chunk_size": [5, 10, 5],
      "chunk_interval": 10,
      "max_connections": 5,
      "timeout": 30
    }
  },
  //tts配置
  "tts": {
    "provider": "xiaozhi",                  //选择tts的类型 doubao, doubao_ws, cosyvoice, xiaozhi等
    "doubao": {
      "appid": "6886011847",
      "access_token": "access_token",       //需要修改为自己的
      "cluster": "volcano_tts",
      "voice": "BV001_streaming",
      "api_url": "https://openspeech.bytedance.com/api/v1/tts",
      "authorization": "Bearer;"
    },
    "doubao_ws": {
      "appid":        "6886011847",         //需要修改为自己的
      "access_token": "access_token",       //需要修改为自己的
      "cluster":      "volcano_tts",        //貌似不用改
      "voice":        "zh_female_wanwanxiaohe_moon_bigtts", //音色
      "ws_host":      "openspeech.bytedance.com",           //服务器地址
      "use_stream":   true
    },
    "cosyvoice": {
      "api_url": "https://tts.linkerai.top/tts", //地址
      "spk_id": "spk_id",                        //音色
      "frame_duration": 60,                      
      "target_sr": 24000,
      "audio_format": "mp3",
      "instruct_text": "你好"
    },
    "edge": {
      "voice": "zh-CN-XiaoxiaoNeural",
      "rate": "+0%",
      "volume": "+0%",
      "pitch": "+0Hz",
      "connect_timeout": 10,
      "receive_timeout": 60
    },
    "xiaozhi": {
      "server_addr": "wss://api.tenclass.net/xiaozhi/v1/",
      "device_id": "ba:8f:17:de:94:94",
      "client_id": "e4b0c442-98fc-4e1b-8c3d-6a5b6a5b6a6d",
      "token": "test-token"
    }
  },
  "llm": {
    "provider": "qwen_72b",
    "deepseek": {
      "type": "openai",
      "model_name": "Pro/deepseek-ai/DeepSeek-V3",
      "api_key": "api_key",
      "base_url": "https://api.siliconflow.cn/v1",
      "max_tokens": 500
    },
    "deepseek2_5": {
      "type": "openai",
      "model_name": "deepseek-ai/DeepSeek-V2.5",
      "api_key": "api_key",
      "base_url": "https://api.siliconflow.cn/v1",
      "max_tokens": 500
    },
    "qwen_72b": {
      "type": "openai",
      "model_name": "Qwen/Qwen2.5-72B-Instruct",
      "api_key": "api_key",
      "base_url": "https://api.siliconflow.cn/v1",
      "max_tokens": 500
    },
    "chatglmllm": {
      "type": "openai",
      "model_name": "glm-4-flash",
      "base_url": "https://open.bigmodel.cn/api/paas/v4/",
      "api_key": "api_key",
      "max_tokens": 500
    }
  },
  //ota接口返回的信息
  "ota": {
    "test": {
      "websocket": {
        "url": "ws://192.168.208.214:8989/xiaozhi/v1/"
      },
      "mqtt": {
        "endpoint": "192.168.208.214"
      }
    },
    "external": {
      "websocket": {
        "url": "wss://www.youdomain.cn/go_ws/xiaozhi/v1/"
      },
      "mqtt": {
        "endpoint": "www.youdomain.cn"
      }
    }
  },
  "wakeup_words": ["小智", "小知", "你好小智"],
  "enable_greeting": true
}
```