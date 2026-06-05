package agentruntime

// Option configures a Runtime instance.
type Option func(*Runtime) error

// WithLLMClient sets the LLM client adapter. Required.
func WithLLMClient(client LLMClient) Option {
	return func(r *Runtime) error {
		r.llmClient = client
		return nil
	}
}

// WithToolGateway sets the tool gateway adapter. Required when tools are used.
func WithToolGateway(gw ToolGateway) Option {
	return func(r *Runtime) error {
		r.toolGateway = gw
		return nil
	}
}

// WithTools registers the available tool descriptors.
func WithTools(tools []ToolDescriptor) Option {
	return func(r *Runtime) error {
		r.tools = tools
		return nil
	}
}

// WithConfig overrides the default configuration.
func WithConfig(config RuntimeConfig) Option {
	return func(r *Runtime) error {
		r.config = config
		return nil
	}
}

// WithExperienceProvider sets the experience provider adapter.
func WithExperienceProvider(provider ExperienceProvider) Option {
	return func(r *Runtime) error {
		r.experienceProvider = provider
		return nil
	}
}

// WithHooks registers one or more hook sinks.
func WithHooks(sinks ...HookSink) Option {
	return func(r *Runtime) error {
		r.hookSinks = append(r.hookSinks, sinks...)
		return nil
	}
}

// WithPromptProvider sets a custom prompt provider.
func WithPromptProvider(provider PromptProvider) Option {
	return func(r *Runtime) error {
		r.promptProvider = provider
		return nil
	}
}

// WithToolPolicy sets a custom tool policy evaluator.
func WithToolPolicy(policy ToolPolicy) Option {
	return func(r *Runtime) error {
		r.toolPolicy = policy
		return nil
	}
}

// WithRouter sets a custom task router for intelligent prompt routing.
func WithRouter(router TaskRouter) Option {
	return func(r *Runtime) error {
		r.router = router
		return nil
	}
}

// WithClock sets a custom clock for testability.
func WithClock(clock Clock) Option {
	return func(r *Runtime) error {
		r.clock = clock
		return nil
	}
}

// WithIDGenerator sets a custom ID generator for testability.
func WithIDGenerator(gen IDGenerator) Option {
	return func(r *Runtime) error {
		r.idGen = gen
		return nil
	}
}
