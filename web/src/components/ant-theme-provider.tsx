"use client";

import type { ReactNode } from "react";
import { ProConfigProvider } from "@ant-design/pro-components";
import { App, ConfigProvider, theme as antdTheme } from "antd";
import zhCN from "antd/locale/zh_CN";

import { useThemeStore } from "@/stores/use-theme-store";

export function AntThemeProvider({ children }: { children: ReactNode }) {
  const theme = useThemeStore((state) => state.theme);
  const dark = theme === "dark";
  const colors = dark
    ? {
        bg: "#1f1d1a",
        layout: "#181715",
        panel: "#24211e",
        elevated: "#292524",
        fill: "#322e29",
        fillHover: "#3a3631",
        border: "#44403c",
        borderSoft: "rgba(214, 211, 209, 0.18)",
        text: "#f5f5f4",
        textSecondary: "#d6d3d1",
        textTertiary: "#a8a29e",
        primary: "#f5f5f4",
        primaryText: "#1c1917",
        menuSelected: "#3a3631",
        tableHeader: "#2b2521",
      }
    : {
        bg: "#fbfaf7",
        layout: "#f4f2ed",
        panel: "#ffffff",
        elevated: "#ffffff",
        fill: "#f5f5f4",
        fillHover: "#e7e5df",
        border: "#d6d3ca",
        borderSoft: "rgba(87, 83, 78, 0.18)",
        text: "#292524",
        textSecondary: "#57534e",
        textTertiary: "#78716c",
        primary: "#111111",
        primaryText: "#ffffff",
        menuSelected: "#f5f5f4",
        tableHeader: "#f8fafc",
      };

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: colors.primary,
          colorPrimaryHover: dark ? "#ffffff" : "#2a2a2a",
          colorPrimaryActive: dark ? "#e7e5e4" : "#000000",
          colorInfo: colors.primary,
          colorBgBase: colors.bg,
          colorBgLayout: colors.layout,
          colorBgContainer: colors.panel,
          colorBgElevated: colors.elevated,
          colorFill: colors.fill,
          colorFillSecondary: colors.fill,
          colorFillTertiary: colors.fillHover,
          colorFillQuaternary: dark ? "rgba(245, 245, 244, 0.08)" : "rgba(28, 25, 23, 0.04)",
          colorBorder: colors.border,
          colorBorderSecondary: colors.borderSoft,
          colorSplit: colors.borderSoft,
          colorText: colors.text,
          colorTextBase: colors.text,
          colorTextSecondary: colors.textSecondary,
          colorTextTertiary: colors.textTertiary,
          colorTextQuaternary: dark ? "#78716c" : "#a8a29e",
          colorIcon: colors.textSecondary,
          colorIconHover: colors.text,
          colorLink: colors.text,
          colorLinkHover: colors.text,
          colorBgSpotlight: dark ? "#f5f5f4" : "#1c1917",
          colorTextLightSolid: dark ? "#1c1917" : "#ffffff",
          colorError: "#ff4d4f",
          colorErrorHover: "#ff7875",
          colorErrorActive: "#d9363e",
          colorWarning: "#faad14",
          colorBgMask: dark ? "rgba(0, 0, 0, 0.62)" : "rgba(28, 25, 23, 0.35)",
          borderRadius: 8,
          borderRadiusLG: 12,
          boxShadow: dark ? "0 18px 48px rgba(0, 0, 0, 0.46)" : "0 18px 48px rgba(41, 37, 36, 0.16)",
          boxShadowSecondary: dark ? "0 0 0 1px rgba(255,255,255,.06)" : "0 1px 2px rgba(15,23,42,.04)",
          fontFamily: '"SF Pro Text","PingFang SC","Microsoft YaHei","Helvetica Neue",sans-serif',
        },
        components: {
          Button: {
            primaryColor: colors.primaryText,
            defaultColor: colors.text,
            defaultBg: dark ? "#1f1d1a" : "#ffffff",
            defaultBorderColor: colors.border,
            defaultHoverColor: colors.text,
            defaultHoverBg: colors.fillHover,
            defaultHoverBorderColor: dark ? "#78716c" : "#a8a29e",
            dangerColor: "#ffffff",
            primaryShadow: "none",
            defaultShadow: "none",
            dangerShadow: "none",
          },
          Modal: {
            contentBg: colors.elevated,
            headerBg: colors.elevated,
            footerBg: colors.elevated,
            titleColor: colors.text,
          },
          Menu: {
            popupBg: colors.elevated,
            itemBg: colors.panel,
            itemColor: colors.textSecondary,
            itemHoverBg: colors.fillHover,
            itemHoverColor: colors.text,
            itemActiveBg: colors.menuSelected,
            itemSelectedBg: colors.menuSelected,
            itemSelectedColor: dark ? "#ffffff" : colors.text,
            darkItemBg: colors.panel,
            darkItemColor: colors.textSecondary,
            darkItemHoverBg: colors.fillHover,
            darkItemHoverColor: colors.text,
            darkItemSelectedBg: colors.menuSelected,
            darkItemSelectedColor: "#ffffff",
          },
          Layout: {
            bodyBg: colors.layout,
            headerBg: colors.panel,
            headerColor: colors.text,
            lightSiderBg: colors.panel,
            siderBg: colors.panel,
          },
          Card: {
            bodyPadding: 24,
            headerBg: colors.panel,
            headerHeight: 56,
          },
          Table: {
            borderColor: colors.borderSoft,
            cellPaddingBlockMD: 14,
            headerBg: colors.tableHeader,
            headerColor: colors.text,
            rowHoverBg: colors.fill,
          },
          Segmented: {
            trackBg: colors.fill,
            itemSelectedBg: colors.primary,
            itemSelectedColor: colors.primaryText,
          },
          Select: {
            selectorBg: dark ? "#1f1d1a" : "#ffffff",
            optionActiveBg: colors.fillHover,
            optionSelectedBg: dark ? "#3f3a35" : "#f5f5f4",
            optionSelectedColor: colors.text,
            activeBorderColor: dark ? "#78716c" : "#a8a29e",
            hoverBorderColor: dark ? "#57534e" : "#a8a29e",
            activeOutlineColor: "transparent",
          },
          Input: {
            activeBg: dark ? "#1f1d1a" : "#ffffff",
            hoverBg: dark ? "#1f1d1a" : "#ffffff",
            activeBorderColor: dark ? "#78716c" : "#a8a29e",
            hoverBorderColor: dark ? "#57534e" : "#a8a29e",
            activeShadow: "none",
          },
          InputNumber: {
            activeBg: dark ? "#1f1d1a" : "#ffffff",
            hoverBg: dark ? "#1f1d1a" : "#ffffff",
            activeBorderColor: dark ? "#78716c" : "#a8a29e",
            hoverBorderColor: dark ? "#57534e" : "#a8a29e",
            activeShadow: "none",
          },
          Tabs: {
            itemColor: colors.textSecondary,
            itemSelectedColor: colors.text,
            itemHoverColor: colors.text,
            inkBarColor: colors.text,
          },
          Checkbox: {
            colorPrimary: colors.primary,
            colorPrimaryHover: colors.primary,
          },
        },
      }}
    >
      <ProConfigProvider dark={dark}>
        <App>{children}</App>
      </ProConfigProvider>
    </ConfigProvider>
  );
}
