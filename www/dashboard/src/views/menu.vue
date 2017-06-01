<style>
	.layout-logo-left{
        width: 100%;
        height: 80px;
        padding: 10px;
       	text-align: center;
    }
    .layout-logo-left img{
    		height: 100%;
    }
    .ivu-menu-item{
    		text-align: center;
    		vertical-align: middle;
    }
  	.ivu-menu-item img{
  		width: 16px;
  	}
</style>
<template>
	<Menu active-name="home" theme="dark" width="auto">
		<div class="layout-logo-left">
			<img src="../imgs/iotalking.png" />
		</div>
		<Menu-item name="home">
			<router-link to="/"><i class="icon iconfont icon-home">主页</i></router-link>
		</Menu-item>
		<Menu-item name="charts">
			<router-link to="/charts"><i class="icon iconfont icon-chart">图表</i></router-link>
		</Menu-item>
		<Menu-item name="about">
			<router-link to="/about"><i class="icon iconfont icon-about">关于</i></router-link>
		</Menu-item>
	</Menu>
</template>
<script>
	import '../font/iconfont.css'
	export default {
		methods: {}
	};
</script>