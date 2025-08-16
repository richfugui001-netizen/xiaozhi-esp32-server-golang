-- 为智能体表添加LLM和TTS配置关联字段
ALTER TABLE agents ADD COLUMN llm_config_id INTEGER;
ALTER TABLE agents ADD COLUMN tts_config_id INTEGER;

-- 添加索引
CREATE INDEX idx_agents_llm_config_id ON agents(llm_config_id);
CREATE INDEX idx_agents_tts_config_id ON agents(tts_config_id);

-- 添加外键约束（可选，根据需要启用）
-- ALTER TABLE agents ADD CONSTRAINT fk_agents_llm_config FOREIGN KEY (llm_config_id) REFERENCES configs(id);
-- ALTER TABLE agents ADD CONSTRAINT fk_agents_tts_config FOREIGN KEY (tts_config_id) REFERENCES configs(id);