# Vestigial v2.2 task configs — kept for /classify endpoint and capability routing.
# The v5 agentic loop does NOT use these for execution.
from configs.research import config as research_config
from configs.summarize import config as summarize_config
from configs.extract import config as extract_config
from configs.write import config as write_config
from configs.translate import config as translate_config
from configs.analyze import config as analyze_config
from configs.assistant import config as assistant_config

CONFIGS = {"research":research_config,"summarize":summarize_config,"extract":extract_config,
    "write":write_config,"translate":translate_config,"analyze":analyze_config,"assistant":assistant_config}

def load_config(capability): return CONFIGS.get(capability, assistant_config)
