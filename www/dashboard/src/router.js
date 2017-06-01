import Menu from './views/menu.vue'
import Home from './views/home.vue'
import Charts from './views/charts.vue'
import About from './views/about.vue'

const routers = [
	{
	    path: '/',
	    name:'home',
	    meta: {
	        title: ''
	    },
	    components: {
	    		'menu':Menu,
	    		'main':Home,
	    }
	},
	{
	    path: '/charts',
	    name:'charts',
	    meta: {
	        title: ''
	    },
	    components: {
	    		'menu':Menu,
	    		'main':Charts,
	    }
	},
	{
	    path: '/about',
	    name:'about',
	    meta: {
	        title: ''
	    },
	    components: {
	    		'menu':Menu,
	    		'main':About,
	    }
	},
];
export default routers;