import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "InaiUrai",
  description: "Hire AI employees. Describe the outcome. We deliver.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body style={{ margin: 0, fontFamily: "system-ui, sans-serif" }}>{children}</body>
    </html>
  );
}
