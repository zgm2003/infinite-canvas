import type { Metadata } from "next";
import { AntThemeProvider } from "@/components/ant-theme-provider";
import { QueryProvider } from "@/components/query-provider";
import { ThemeSync } from "@/components/theme-sync";
import { UserSessionSync } from "@/components/user-session-sync";
import "antd/dist/reset.css";
import "./globals.css";

export const metadata: Metadata = {
  title: "无限画布",
  description: "一个无限画布创作工具",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning className="font-sans">
      <body
        className="bg-background text-foreground antialiased"
        style={{
          fontFamily:
            '"SF Pro Display","SF Pro Text","PingFang SC","Microsoft YaHei","Helvetica Neue",sans-serif',
        }}
      >
        <script
          dangerouslySetInnerHTML={{
            __html: `try{var s=JSON.parse(localStorage.getItem("infinite-canvas:theme_store")||"{}");var t=s.state&&s.state.theme==="light"?"light":"dark";document.documentElement.classList.toggle("dark",t==="dark");document.documentElement.style.colorScheme=t}catch(e){}`,
          }}
        />
        <ThemeSync />
        <AntThemeProvider>
          <QueryProvider>
            <UserSessionSync />
            {children}
          </QueryProvider>
        </AntThemeProvider>
      </body>
    </html>
  );
}
