<style>
	.layout{
		height: 100%;
		width: 100%;
        border: 1px solid #d7dde4;
        background: #f0f0f0;
        position: absolute;
    }
    .row{
    		height: 100%;
    }
    .layout-content{
        position: absolute;
        top: 40px;
        left: 0;
        right: 0;
        bottom: 30px;
        overflow: scroll;
        
        border-radius: 4px;
    }
    .layout-copy{
    		position: absolute;
    		height: 25px;
        left: 0;
        right: 0;
        bottom: 0px;
        margin-left: 10px;
    }
    .layout-header{
    		padding: 10px;
    		background: #464c5b;
    }
    .layout-header span{
    		color: white;
        font-weight: bolder;
    }
    .layout-content-main{
        position: absolute;
        top: 10px;
        left: 10px;
        right: 10px;
        bottom: 10px;
    }
    .layout-menu-left{
        background: #464c5b;
        color: white;
        font-weight: bolder;
    }
    
    
</style>
<template>
	<div class="layout">
        <Row type="flex" class="row">
            <i-col span="3" class="layout-menu-left">
                 <router-view name="menu"></router-view>
            </i-col>
            <i-col span="21">
            		<div class="layout-header">
            			<span>Iotalking | Public broker | MQTT dashboard</span>
            		</div>
                <div class="layout-content">
                    <div class="layout-content-main">
                    		<router-view name="main" :data="test"></router-view>
                    </div>
                </div>
                <div class="layout-copy">
                    2017- copy; Iotalking
                </div>
            </i-col>
        </Row>
   </div>
</template>
<script>
    export default {
        data() {
            return {
				test: {
					test:'23423',
				},
            };
        },
        mounted() {

        },
        beforeDestroy() {

        },
        methods: {

        }
    };
</script>