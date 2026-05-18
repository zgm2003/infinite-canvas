"use client";

import { Select } from "antd";

import type { AiConfig } from "@/lib/ai-config";

type ModelPickerProps = {
  config: AiConfig;
  value?: string;
  onChange: (model: string) => void;
  className?: string;
  fullWidth?: boolean;
  placeholder?: string;
  onMissingConfig?: () => void;
};

export function ModelPicker({ config, value, onChange, className, fullWidth = false, placeholder = "选择模型", onMissingConfig }: ModelPickerProps) {
  const options = Array.from(new Set([value, ...config.models].filter(Boolean))).map((model) => ({ value: model, label: model }));
  const width = fullWidth ? "100%" : `min(${Math.max(156, (value || placeholder).length * 8 + 64)}px, 100%)`;

  return (
    <Select
      showSearch
      className={`canvas-control-select ${className || ""}`}
      popupMatchSelectWidth={false}
      style={{ width, maxWidth: "100%", minWidth: 0, flexShrink: 1 }}
      value={value || undefined}
      placeholder={placeholder}
      options={options}
      notFoundContent="请先到配置里拉取模型列表"
      onChange={onChange}
      onMouseDown={(event) => event.stopPropagation()}
      onClick={() => {
        if (!options.length) onMissingConfig?.();
      }}
      filterOption={(input, option) => String(option?.label || "").toLowerCase().includes(input.toLowerCase())}
    />
  );
}
