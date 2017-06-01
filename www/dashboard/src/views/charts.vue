<style scoped>
	.ivu-row{
		position: relative;
		/*border:1px solid blue;*/
	}
	.ivu-col{
		position: relative;
		height: 100%;
	}
	.box {
		/*border:1px solid red;*/
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		margin: 10px;
		padding: 5px;
		background: white;
	}
	
	.box-title {
		margin-bottom: 15px;
	}
	
	.r1 {
		height: 100px;
	}
	
	.r2 {
		height: 200px;
	}
	
	.r3 {
		height: 300px;
	}
	
	.r4 {
		height: 400px;
	}
	
	.r5 {
		min-height: 500px;
	}
	.r6 {
		height: 600px;
	}
</style>
<template>
	<Row>
		<Row class="r3">
			<Col span="24">
				<div class="box">
					<h1 class="box-title">Broker</h1>
					<p>这是Iotalking broker的数据看板。<br>你可以使用任意的MQTT客户端连接到本Broker</p>
				</div>
			</Col>
		</Row>
		<Row class="r6">
			<Col span="12">
				<div class="box">
				
				</div>
			</Col>
			<Col span="12">
				<div class="box">
				
				</div>
			</Col>		
		</Row>
	</Row>
</template>
<script>
	export default {
		methods: {

		}
	};
</script>