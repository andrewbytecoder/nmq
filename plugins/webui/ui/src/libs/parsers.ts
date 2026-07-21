export const parseMiddlewareType = (middleware: Record<string, unknown>): string | undefined => {
  const plugin = middleware.plugin

  if (plugin && typeof plugin === 'object' && !Array.isArray(plugin)) {
    const [pluginName] = Object.keys(plugin as Record<string, unknown>)

    if (pluginName) {
      return pluginName
    }
  }

  return typeof middleware.type === 'string' ? middleware.type : undefined
}
