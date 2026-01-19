# kunpeng-tap BDD 测试步骤定义

# 导入所有步骤定义，使 pytest-bdd 能够自动发现它们
from .topology_aware_steps import *  # noqa

# kunpeng_tap_steps 依赖通用框架，暂时不导入
# from .kunpeng_tap_steps import *  # noqa
